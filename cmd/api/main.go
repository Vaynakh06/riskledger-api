package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"riskledger/internal/auth"
	"riskledger/internal/config"
	"riskledger/internal/service"
	"riskledger/internal/store"
)

type App struct {
	service *service.Service
	secret  string
	ready   func(context.Context) error
}

type contextKey string

const userIDKey contextKey = "user_id"

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	var repository service.Store
	var closeStore func() error
	if cfg.DemoMode {
		memoryStore := store.NewInMemoryStore()
		memoryStore.SeedDemoData()
		repository = memoryStore
	} else {
		db, err := sql.Open("pgx", cfg.DatabaseURL)
		if err != nil {
			log.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = db.PingContext(ctx)
		cancel()
		if err != nil {
			_ = db.Close()
			log.Fatal(err)
		}
		postgresStore := store.NewPostgresStore(db)
		repository = postgresStore
		closeStore = postgresStore.Close
	}
	if closeStore != nil {
		defer closeStore()
	}
	app := &App{service: service.NewService(repository), secret: cfg.JWTSecret, ready: func(context.Context) error { return nil }}
	if postgresStore, ok := repository.(*store.PostgresStore); ok {
		app.ready = postgresStore.Ping
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", app.healthHandler)
	mux.HandleFunc("/health/ready", app.readyHandler)
	mux.HandleFunc("/api/v1/auth/register", app.registerHandler)
	mux.HandleFunc("/api/v1/auth/login", app.loginHandler)
	mux.Handle("/", http.FileServer(http.Dir("web")))
	mux.Handle("/api/v1/portfolios", app.requireAuth(http.HandlerFunc(app.portfoliosHandler)))
	mux.Handle("/api/v1/portfolios/", app.requireAuth(http.HandlerFunc(app.portfolioDetailHandler)))
	mux.Handle("/api/v1/trades", app.requireAuth(http.HandlerFunc(app.tradesHandler)))
	mux.Handle("/api/v1/trades/import", app.requireAuth(http.HandlerFunc(app.importTradesHandler)))
	mux.Handle("/api/v1/positions", app.requireAuth(http.HandlerFunc(app.positionsHandler)))
	mux.Handle("/api/v1/analytics", app.requireAuth(http.HandlerFunc(app.analyticsHandler)))
	mux.Handle("/api/v1/summary", app.requireAuth(http.HandlerFunc(app.summaryHandler)))
	mux.Handle("/api/v1/risk-limits", app.requireAuth(http.HandlerFunc(app.riskLimitsHandler)))
	mux.Handle("/api/v1/risk-evaluate", app.requireAuth(http.HandlerFunc(app.riskEvaluateHandler)))

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	log.Printf("RiskLedger API running on :%s", cfg.Port)
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	case <-stop:
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
		}
	}
}

func (a *App) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (a *App) readyHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := a.ready(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (a *App) registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	user, err := a.service.Register(req.Email, req.Password)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, user)
}

func (a *App) loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	user, err := a.service.Login(req.Email, req.Password)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}
	token, err := auth.IssueToken(a.secret, user.ID, user.Email, 15*time.Minute)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not issue token"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "token": token, "expires_in": 900})
}

func (a *App) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing bearer token"})
			return
		}
		claims, err := auth.ParseToken(a.secret, strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired token"})
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey, claims.Subject)))
	})
}

func userIDFromRequest(r *http.Request) (string, bool) {
	userID, ok := r.Context().Value(userIDKey).(string)
	return userID, ok && userID != ""
}

func (a *App) firstPortfolioID(r *http.Request) (string, bool) {
	userID, ok := userIDFromRequest(r)
	if !ok {
		return "", false
	}
	portfolios := a.service.PortfoliosForUser(userID)
	if len(portfolios) == 0 {
		return "", false
	}
	return portfolios[0].ID, true
}

func (a *App) portfolioIDForRequest(r *http.Request) (string, bool) {
	userID, ok := userIDFromRequest(r)
	if !ok {
		return "", false
	}
	requestedID := r.URL.Query().Get("portfolio_id")
	if requestedID == "" {
		return a.firstPortfolioID(r)
	}
	portfolio, exists := a.service.PortfolioByID(requestedID)
	return requestedID, exists && portfolio.UserID == userID
}

func (a *App) portfoliosHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, a.service.PortfoliosForUser(userID))
		return
	}
	if r.Method == http.MethodPost {
		var req struct {
			Name         string `json:"name"`
			BaseCurrency string `json:"base_currency"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
			return
		}
		portfolio, err := a.service.CreatePortfolio(userID, req.Name, req.BaseCurrency)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, portfolio)
		return
	}
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}

func (a *App) portfolioDetailHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "portfolio detail endpoint"})
}

func (a *App) tradesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		portfolioID, ok := a.portfolioIDForRequest(r)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "portfolio not found"})
			return
		}
		writeJSON(w, http.StatusOK, a.service.TradesForPortfolio(portfolioID))
		return
	}
	if r.Method == http.MethodPost {
		var req struct {
			PortfolioID string   `json:"portfolio_id"`
			Instrument  string   `json:"instrument"`
			Side        string   `json:"side"`
			Quantity    float64  `json:"quantity"`
			Price       float64  `json:"price"`
			Fee         float64  `json:"fee"`
			Strategy    string   `json:"strategy"`
			Notes       string   `json:"notes"`
			Tags        []string `json:"tags"`
			ChecklistOK bool     `json:"checklist_ok"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
			return
		}
		userID, authenticated := userIDFromRequest(r)
		portfolio, ownsPortfolio := a.service.PortfolioByID(req.PortfolioID)
		if !authenticated || !ownsPortfolio || portfolio.UserID != userID {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "portfolio access denied"})
			return
		}
		if err := a.service.AddTrade(req.PortfolioID, req.Instrument, req.Side, req.Quantity, req.Price, req.Fee, req.Strategy, req.Notes, req.Tags, req.ChecklistOK); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"status": "trade recorded"})
		return
	}
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}

func (a *App) importTradesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file is required"})
		return
	}
	portfolioID := r.FormValue("portfolio_id")
	userID, authenticated := userIDFromRequest(r)
	portfolio, owns := a.service.PortfolioByID(portfolioID)
	if !authenticated || !owns || portfolio.UserID != userID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "portfolio access denied"})
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file is required"})
		return
	}
	defer file.Close()
	reader := csv.NewReader(io.LimitReader(file, 5<<20))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil || len(rows) < 2 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid CSV"})
		return
	}
	imported := 0
	errors := make([]string, 0)
	for index, row := range rows[1:] {
		if len(row) < 5 {
			errors = append(errors, "строка "+strconv.Itoa(index+2)+": нужно минимум 5 полей")
			continue
		}
		quantity, quantityErr := strconv.ParseFloat(strings.TrimSpace(row[2]), 64)
		price, priceErr := strconv.ParseFloat(strings.TrimSpace(row[3]), 64)
		fee, feeErr := strconv.ParseFloat(strings.TrimSpace(row[4]), 64)
		if quantityErr != nil || priceErr != nil || feeErr != nil {
			errors = append(errors, "строка "+strconv.Itoa(index+2)+": неверные числа")
			continue
		}
		strategy, notes, tags := "", "", []string{}
		checklist := false
		if len(row) > 5 {
			strategy = strings.TrimSpace(row[5])
		}
		if len(row) > 6 && strings.TrimSpace(row[6]) != "" {
			for _, tag := range strings.Split(row[6], ",") {
				tags = append(tags, strings.TrimSpace(tag))
			}
		}
		if len(row) > 7 {
			notes = strings.TrimSpace(row[7])
		}
		if len(row) > 8 {
			checklist, _ = strconv.ParseBool(strings.TrimSpace(row[8]))
		}
		if err := a.service.AddTrade(portfolioID, strings.TrimSpace(row[0]), strings.TrimSpace(row[1]), quantity, price, fee, strategy, notes, tags, checklist); err != nil {
			errors = append(errors, "строка "+strconv.Itoa(index+2)+": "+err.Error())
			continue
		}
		imported++
	}
	writeJSON(w, http.StatusCreated, map[string]any{"imported": imported, "errors": errors})
}

func (a *App) positionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	portfolioID, ok := a.portfolioIDForRequest(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "portfolio not found"})
		return
	}
	writeJSON(w, http.StatusOK, a.service.Positions(portfolioID))
}

func (a *App) analyticsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	portfolioID, ok := a.portfolioIDForRequest(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "portfolio not found"})
		return
	}
	writeJSON(w, http.StatusOK, a.service.Analytics(portfolioID))
}

func (a *App) summaryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	portfolioID, ok := a.portfolioIDForRequest(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "portfolio not found"})
		return
	}
	writeJSON(w, http.StatusOK, a.service.Summary(portfolioID))
}

func (a *App) riskLimitsHandler(w http.ResponseWriter, r *http.Request) {
	portfolioID, ok := a.portfolioIDForRequest(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "portfolio not found"})
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, a.service.RiskLimits(portfolioID))
		return
	}
	if r.Method == http.MethodPost {
		var req struct {
			Type  string  `json:"type"`
			Value float64 `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
			return
		}
		limit, err := a.service.AddRiskLimit(portfolioID, req.Type, req.Value)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, limit)
		return
	}
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}

func (a *App) riskEvaluateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	portfolioID, ok := a.portfolioIDForRequest(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "portfolio not found"})
		return
	}
	writeJSON(w, http.StatusOK, a.service.EvaluateRisk(portfolioID))
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
