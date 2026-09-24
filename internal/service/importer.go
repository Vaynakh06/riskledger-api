package service

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"riskledger/internal/domain"
)

type brokerRow struct {
	ExternalID string
	Kind       string
	Instrument string
	Side       string
	Quantity   float64
	Price      float64
	Fee        float64
	Amount     float64
	Currency   string
	ExecutedAt time.Time
	Raw        string
}

func (s *Service) ImportBroker(portfolioID, source, filename string, data []byte) (domain.ImportBatch, error) {
	if _, ok := s.store.PortfolioByID(portfolioID); !ok {
		return domain.ImportBatch{}, fmt.Errorf("portfolio not found")
	}
	if strings.TrimSpace(source) == "" {
		source = "generic"
	}
	rows, err := parseBrokerFile(filename, data)
	if err != nil {
		return domain.ImportBatch{}, err
	}
	batch := domain.NewImportBatch(portfolioID, strings.ToLower(strings.TrimSpace(source)), filename)
	batch.Total = len(rows)
	s.store.AddImportBatch(batch)

	for _, row := range rows {
		record := domain.NewImportRecord(batch.ID, batch.Source, row.Kind)
		record.ExternalID, record.Instrument, record.Side = row.ExternalID, row.Instrument, row.Side
		record.Quantity, record.Price, record.Fee, record.Amount = row.Quantity, row.Price, row.Fee, row.Amount
		record.Currency, record.ExecutedAt, record.Raw = row.Currency, row.ExecutedAt, row.Raw
		if row.ExternalID != "" && s.store.ImportRecordExists(portfolioID, batch.Source, row.ExternalID) {
			record.Status = "duplicate"
			record.Reason = "transaction was already imported"
			batch.Unmatched++
		} else if err := s.reconcileImportedRow(portfolioID, &record, row); err != nil {
			record.Reason = err.Error()
			if strings.Contains(err.Error(), "exceeds open position") || strings.Contains(err.Error(), "unsupported operation") {
				record.Status = "needs_review"
				batch.Review++
			} else {
				record.Status = "unmatched"
				batch.Unmatched++
			}
		} else {
			record.Status = "matched"
			batch.Matched++
		}
		s.store.AddImportRecord(record)
	}
	s.store.AddImportBatch(batch)
	return batch, nil
}

func (s *Service) reconcileImportedRow(portfolioID string, record *domain.ImportRecord, row brokerRow) error {
	if row.Kind == "trade" {
		if row.Instrument == "" || (row.Side != "buy" && row.Side != "sell") || row.Quantity <= 0 || row.Price <= 0 {
			return fmt.Errorf("trade needs instrument, side, quantity and price")
		}
		if row.Side == "sell" && calculateOpenPosition(s.store.TradesForPortfolio(portfolioID), row.Instrument) < row.Quantity {
			return fmt.Errorf("sell quantity exceeds open position")
		}
		return s.AddTradeWithMetaAt(portfolioID, row.Instrument, row.Side, row.Quantity, row.Price, max(row.Fee, 0), "", "broker import", nil, false, "broker:"+record.Source, "broker import", row.ExecutedAt)
	}
	if row.Kind == "fee" || row.Kind == "dividend" {
		if row.Amount == 0 {
			return fmt.Errorf("%s needs a non-zero amount", row.Kind)
		}
		entry := domain.NewLedgerEntry(portfolioID, row.Kind, row.Instrument, "", row.Quantity, row.Price, row.Amount, row.Currency, "broker import", "external_id="+row.ExternalID, "broker:"+record.Source)
		s.store.AddLedgerEntry(entry)
		return nil
	}
	return fmt.Errorf("unsupported operation type")
}

func (s *Service) ImportBatches(portfolioID string) []domain.ImportBatch {
	return s.store.ImportBatchesForPortfolio(portfolioID)
}

func (s *Service) ImportBatchByID(batchID string) (domain.ImportBatch, bool) {
	return s.store.ImportBatchByID(batchID)
}

func (s *Service) ImportRecords(batchID string) []domain.ImportRecord {
	return s.store.ImportRecordsForBatch(batchID)
}

func parseBrokerFile(filename string, data []byte) ([]brokerRow, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	var rows [][]string
	var err error
	if ext == ".xlsx" {
		rows, err = readXLSX(data)
	} else {
		rows, err = readCSV(data)
	}
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("import file must include a header and at least one row")
	}
	headers := make([]string, len(rows[0]))
	for i, header := range rows[0] {
		headers[i] = normalizeHeader(header)
	}
	result := make([]brokerRow, 0, len(rows)-1)
	for _, values := range rows[1:] {
		if len(strings.TrimSpace(strings.Join(values, ""))) == 0 {
			continue
		}
		fields := make(map[string]string, len(headers))
		for i, header := range headers {
			if i < len(values) {
				fields[header] = strings.TrimSpace(values[i])
			}
		}
		result = append(result, normalizeBrokerRow(fields, values))
	}
	return result, nil
}

func normalizeBrokerRow(fields map[string]string, raw []string) brokerRow {
	kind := strings.ToLower(first(fields, "type", "operation", "transaction", "activity"))
	switch {
	case strings.Contains(kind, "dividend"), strings.Contains(kind, "distribution"):
		kind = "dividend"
	case strings.Contains(kind, "fee"), strings.Contains(kind, "commission"), strings.Contains(kind, "charge"):
		kind = "fee"
	default:
		kind = "trade"
	}
	side := strings.ToLower(first(fields, "side", "action", "direction", "buy_sell"))
	if strings.Contains(side, "sell") || strings.Contains(side, "sale") {
		side = "sell"
	} else if strings.Contains(side, "buy") || strings.Contains(side, "purchase") {
		side = "buy"
	}
	amount := parseNumber(first(fields, "amount", "value", "net_amount", "total"))
	fee := parseNumber(first(fields, "fee", "fees", "commission"))
	if kind == "fee" && amount == 0 {
		amount = -fee
	}
	currency := first(fields, "currency", "ccy", "base_currency")
	if currency == "" {
		currency = "USD"
	}
	return brokerRow{
		ExternalID: first(fields, "id", "trade_id", "transaction_id", "order_id", "reference"),
		Kind:       kind, Instrument: strings.ToUpper(first(fields, "instrument", "symbol", "ticker", "asset")), Side: side,
		Quantity: parseNumber(first(fields, "quantity", "qty", "units", "shares")), Price: parseNumber(first(fields, "price", "execution_price", "unit_price")),
		Fee: fee, Amount: amount, Currency: currency,
		ExecutedAt: parseDate(first(fields, "date", "datetime", "executed_at", "timestamp", "time")), Raw: strings.Join(raw, " | "),
	}
}

func readCSV(data []byte) ([][]string, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	return reader.ReadAll()
}

type xlsxCell struct {
	Ref   string `xml:"r,attr"`
	Type  string `xml:"t,attr"`
	Value string `xml:"v"`
}
type xlsxRow struct {
	Cells []xlsxCell `xml:"c"`
}
type xlsxSheet struct {
	Rows []xlsxRow `xml:"sheetData>row"`
}
type xlsxShared struct {
	Items []struct {
		Text string `xml:"t"`
	} `xml:"si"`
}

func readXLSX(data []byte) ([][]string, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("invalid xlsx file: %w", err)
	}
	shared := []string{}
	for _, file := range archive.File {
		if file.Name != "xl/sharedStrings.xml" {
			continue
		}
		content, readErr := readZipFile(file)
		if readErr != nil {
			return nil, readErr
		}
		var values xlsxShared
		if xml.Unmarshal(content, &values) == nil {
			for _, item := range values.Items {
				shared = append(shared, item.Text)
			}
		}
	}
	for _, file := range archive.File {
		if !strings.HasPrefix(file.Name, "xl/worksheets/") || !strings.HasSuffix(file.Name, ".xml") {
			continue
		}
		content, readErr := readZipFile(file)
		if readErr != nil {
			return nil, readErr
		}
		var sheet xlsxSheet
		if err := xml.Unmarshal(content, &sheet); err != nil {
			return nil, fmt.Errorf("invalid worksheet: %w", err)
		}
		out := make([][]string, 0, len(sheet.Rows))
		for _, row := range sheet.Rows {
			values := make([]string, len(row.Cells))
			for i, cell := range row.Cells {
				values[i] = cell.Value
				if cell.Type == "s" {
					index, _ := strconv.Atoi(cell.Value)
					if index >= 0 && index < len(shared) {
						values[i] = shared[index]
					}
				}
			}
			out = append(out, values)
		}
		return out, nil
	}
	return nil, fmt.Errorf("xlsx worksheet not found")
}

func readZipFile(file *zip.File) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(io.LimitReader(reader, 10<<20))
}

func normalizeHeader(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer(" ", "_", "-", "_", "/", "_").Replace(value)
	return value
}
func first(fields map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := fields[key]; value != "" {
			return value
		}
	}
	return ""
}
func firstOr(fields map[string]string, key string, fallback string) string {
	if value := fields[key]; value != "" {
		return value
	}
	return fallback
}
func parseNumber(value string) float64 {
	value = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(value, ",", "."), " ", ""))
	number, _ := strconv.ParseFloat(value, 64)
	return number
}
func parseDate(value string) time.Time {
	for _, layout := range []string{time.RFC3339, "2006-01-02", "02.01.2006", "01/02/2006", "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}
func max(left, right float64) float64 {
	if left > right {
		return left
	}
	return right
}
