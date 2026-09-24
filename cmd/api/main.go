package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"riskledger/internal/auth"
	"riskledger/internal/config"
	"riskledger/internal/domain"
	"riskledger/internal/service"
	"riskledger/internal/store"
)

type App struct {
	service        *service.Service
	secret         string
	ready          func(context.Context) error
	loginLimiter   *loginLimiter
	brokerProvider service.BrokerSyncProvider
}

type loginLimiter struct {
	mu       sync.Mutex
	failures map[string][]time.Time
}

func newLoginLimiter() *loginLimiter { return &loginLimiter{failures: make(map[string][]time.Time)} }

func (l *loginLimiter) allowed(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := time.Now().Add(-15 * time.Minute)
	valid := l.failures[ip][:0]
	for _, failure := range l.failures[ip] {
		if failure.After(cutoff) {
			valid = append(valid, failure)
		}
	}
	l.failures[ip] = valid
	return len(valid) < 10
}

func (l *loginLimiter) failed(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.failures[ip] = append(l.failures[ip], time.Now())
}
func (l *loginLimiter) success(ip string) { l.mu.Lock(); defer l.mu.Unlock(); delete(l.failures, ip) }

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
	priceProvider := service.NewHTTPPriceProvider(cfg.MarketDataURL, cfg.MarketDataAPIKey)
	notifier := service.NewChannelNotifier(service.NotifierConfig{SMTPHost: cfg.SMTPHost, SMTPPort: cfg.SMTPPort, SMTPUser: cfg.SMTPUser, SMTPPassword: cfg.SMTPPassword, AlertFrom: cfg.AlertFrom, TelegramBotToken: cfg.TelegramBotToken, TelegramChatID: cfg.TelegramChatID, SlackWebhookURL: cfg.SlackWebhookURL, AlertWebhookURL: cfg.AlertWebhookURL})
	brokerProvider := service.NewHTTPBrokerProvider(cfg.BrokerAPIURL, cfg.BrokerAPIToken)
	app := &App{service: service.NewServiceWithProviders(repository, priceProvider, notifier), secret: cfg.JWTSecret, ready: func(context.Context) error { return nil }, loginLimiter: newLoginLimiter(), brokerProvider: brokerProvider}
	if postgresStore, ok := repository.(*store.PostgresStore); ok {
		app.ready = postgresStore.Ping
	}
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			app.service.ProcessDeliveryJobs()
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", app.healthHandler)
	mux.HandleFunc("/health/ready", app.readyHandler)
	mux.HandleFunc("/api/v1/auth/register", app.registerHandler)
	mux.HandleFunc("/api/v1/auth/login", app.loginHandler)
	mux.HandleFunc("/api/v1/auth/refresh", app.refreshHandler)
	mux.Handle("/api/v1/auth/2fa/enable", app.requireAuth(http.HandlerFunc(app.enable2FAHandler)))
	mux.Handle("/", http.FileServer(http.Dir("web")))
	mux.Handle("/api/v1/portfolios", app.requireAuth(http.HandlerFunc(app.portfoliosHandler)))
	mux.Handle("/api/v1/portfolios/", app.requireAuth(http.HandlerFunc(app.portfolioDetailHandler)))
	mux.Handle("/api/v1/trades", app.requireAuth(http.HandlerFunc(app.tradesHandler)))
	mux.Handle("/api/v1/trades/import", app.requireAuth(http.HandlerFunc(app.importTradesHandler)))
	mux.Handle("/api/v1/imports", app.requireAuth(http.HandlerFunc(app.importsHandler)))
	mux.Handle("/api/v1/imports/", app.requireAuth(http.HandlerFunc(app.importDetailsHandler)))
	mux.Handle("/api/v1/positions", app.requireAuth(http.HandlerFunc(app.positionsHandler)))
	mux.Handle("/api/v1/analytics", app.requireAuth(http.HandlerFunc(app.analyticsHandler)))
	mux.Handle("/api/v1/summary", app.requireAuth(http.HandlerFunc(app.summaryHandler)))
	mux.Handle("/api/v1/cash", app.requireAuth(http.HandlerFunc(app.cashHandler)))
	mux.Handle("/api/v1/performance-report", app.requireAuth(http.HandlerFunc(app.performanceReportHandler)))
	mux.Handle("/api/v1/benchmarks", app.requireAuth(http.HandlerFunc(app.benchmarksHandler)))
	mux.Handle("/api/v1/allocations", app.requireAuth(http.HandlerFunc(app.allocationsHandler)))
	mux.Handle("/api/v1/fx-rates", app.requireAuth(http.HandlerFunc(app.fxRatesHandler)))
	mux.Handle("/api/v1/ledger", app.requireAuth(http.HandlerFunc(app.ledgerHandler)))
	mux.Handle("/api/v1/risk-snapshot", app.requireAuth(http.HandlerFunc(app.riskSnapshotHandler)))
	mux.Handle("/api/v1/risk-scenario", app.requireAuth(http.HandlerFunc(app.riskScenarioHandler)))
	mux.Handle("/api/v1/risk-alert-rules", app.requireAuth(http.HandlerFunc(app.riskAlertRulesHandler)))
	mux.Handle("/api/v1/risk-alerts/evaluate", app.requireAuth(http.HandlerFunc(app.evaluateAlertsHandler)))
	mux.Handle("/api/v1/risk-alert-events", app.requireAuth(http.HandlerFunc(app.alertEventsHandler)))
	mux.Handle("/api/v1/audit-log", app.requireAuth(http.HandlerFunc(app.auditLogHandler)))
	mux.Handle("/api/v1/broker-sync", app.requireAuth(http.HandlerFunc(app.brokerSyncHandler)))
	mux.Handle("/api/v1/risk-limits", app.requireAuth(http.HandlerFunc(app.riskLimitsHandler)))
	mux.Handle("/api/v1/risk-evaluate", app.requireAuth(http.HandlerFunc(app.riskEvaluateHandler)))

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      securityHeaders(mux),
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
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
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
	ip := clientIP(r)
	if a.loginLimiter != nil && !a.loginLimiter.allowed(ip) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many login attempts"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	user, err := a.service.LoginWithOTP(req.Email, req.Password, req.Code)
	if err != nil {
		if a.loginLimiter != nil {
			a.loginLimiter.failed(ip)
		}
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}
	if a.loginLimiter != nil {
		a.loginLimiter.success(ip)
	}
	token, err := auth.IssueToken(a.secret, user.ID, user.Email, 15*time.Minute)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not issue token"})
		return
	}
	refreshToken, err := auth.IssueToken(a.secret, user.ID, user.Email, 30*24*time.Hour)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not issue refresh token"})
		return
	}
	setRefreshCookie(w, refreshToken, r)
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "token": token, "expires_in": 900, "refresh_expires_in": 30 * 24 * 60 * 60})
}

func (a *App) enable2FAHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	userID, ok := userIDFromRequest(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	url, err := a.service.Enable2FA(userID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"otpauth_url": url})
}

func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		return strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func (a *App) refreshHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	token := ""
	if cookie, cookieErr := r.Cookie("riskledger_refresh"); cookieErr == nil {
		token = cookie.Value
	}
	if token == "" {
		header := r.Header.Get("Authorization")
		if strings.HasPrefix(header, "Bearer ") {
			token = strings.TrimPrefix(header, "Bearer ")
		}
	}
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing refresh token"})
		return
	}
	claims, err := auth.ParseToken(a.secret, token)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "refresh session expired"})
		return
	}
	accessToken, err := auth.IssueToken(a.secret, claims.Subject, claims.Email, 15*time.Minute)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not refresh session"})
		return
	}
	refreshToken, err := auth.IssueToken(a.secret, claims.Subject, claims.Email, 30*24*time.Hour)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not rotate refresh token"})
		return
	}
	setRefreshCookie(w, refreshToken, r)
	writeJSON(w, http.StatusOK, map[string]any{"token": accessToken, "expires_in": 900})
}

func setRefreshCookie(w http.ResponseWriter, token string, r *http.Request) {
	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{Name: "riskledger_refresh", Value: token, Path: "/", MaxAge: 30 * 24 * 60 * 60, HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'")
		next.ServeHTTP(w, r)
	})
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
		if err := a.service.AddTradeWithMeta(req.PortfolioID, req.Instrument, req.Side, req.Quantity, req.Price, req.Fee, req.Strategy, req.Notes, req.Tags, req.ChecklistOK, userID, "trade recorded"); err != nil {
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

func (a *App) importsHandler(w http.ResponseWriter, r *http.Request) {
	portfolioID, ok := a.portfolioIDForRequest(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "portfolio not found"})
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, a.service.ImportBatches(portfolioID))
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file is required"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file is required"})
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 10<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "could not read import file"})
		return
	}
	source := r.FormValue("source")
	batch, err := a.service.ImportBroker(portfolioID, source, header.Filename, data)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, batch)
}

func (a *App) importDetailsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	batchID := strings.TrimPrefix(r.URL.Path, "/api/v1/imports/")
	if batchID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "batch id is required"})
		return
	}
	batch, exists := a.service.ImportBatchByID(batchID)
	portfolio, owns := a.service.PortfolioByID(batch.PortfolioID)
	userID, authenticated := userIDFromRequest(r)
	if !exists || !owns || !authenticated || portfolio.UserID != userID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "import batch not found"})
		return
	}
	writeJSON(w, http.StatusOK, a.service.ImportRecords(batchID))
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

func (a *App) cashHandler(w http.ResponseWriter, r *http.Request) {
	portfolioID, ok := a.portfolioIDForRequest(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "portfolio not found"})
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]any{"balance": a.service.CashBalance(portfolioID), "entries": a.service.CashEntries(portfolioID)})
		return
	}
	var req struct {
		Type     string  `json:"type"`
		Amount   float64 `json:"amount"`
		Currency string  `json:"currency"`
		Reason   string  `json:"reason"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	actor, _ := userIDFromRequest(r)
	entry, err := a.service.AddCash(portfolioID, req.Type, req.Amount, req.Currency, req.Reason, actor)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, entry)
}

func (a *App) performanceReportHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	id, ok := a.portfolioIDForRequest(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "portfolio not found"})
		return
	}
	writeJSON(w, http.StatusOK, a.service.PerformanceReport(id))
}

func (a *App) benchmarksHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := a.portfolioIDForRequest(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "portfolio not found"})
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, a.service.Benchmarks(id))
		return
	}
	var req struct {
		Symbol string `json:"symbol"`
		Name   string `json:"name"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	actor, _ := userIDFromRequest(r)
	item, err := a.service.AddBenchmark(id, req.Symbol, req.Name, actor)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (a *App) allocationsHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := a.portfolioIDForRequest(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "portfolio not found"})
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, a.service.Allocations(id))
		return
	}
	var req struct {
		Instrument string  `json:"instrument"`
		Category   string  `json:"category"`
		Target     float64 `json:"target_weight"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	actor, _ := userIDFromRequest(r)
	item, err := a.service.AddAllocation(id, req.Instrument, req.Category, req.Target, actor)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (a *App) fxRatesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		Base  string  `json:"base"`
		Quote string  `json:"quote"`
		Rate  float64 `json:"rate"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if err := a.service.SetFXRate(req.Base, req.Quote, req.Rate); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "stored"})
}

func (a *App) ledgerHandler(w http.ResponseWriter, r *http.Request) {
	portfolioID, ok := a.portfolioIDForRequest(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "portfolio not found"})
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, a.service.LedgerEntries(portfolioID))
		return
	}
	if r.Method == http.MethodPost {
		var req struct {
			Type       string  `json:"type"`
			Instrument string  `json:"instrument"`
			Side       string  `json:"side"`
			Quantity   float64 `json:"quantity"`
			Price      float64 `json:"price"`
			Currency   string  `json:"currency"`
			Reason     string  `json:"reason"`
			Notes      string  `json:"notes"`
			CreatedBy  string  `json:"created_by"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
			return
		}
		actor, _ := userIDFromRequest(r)
		entry, err := a.service.AddManualLedgerEntry(portfolioID, req.Type, req.Instrument, req.Side, req.Quantity, req.Price, req.Reason, req.Notes, actor)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, entry)
		return
	}
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}

func (a *App) riskSnapshotHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	portfolioID, ok := a.portfolioIDForRequest(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "portfolio not found"})
		return
	}
	writeJSON(w, http.StatusOK, a.service.RiskSnapshot(portfolioID))
}

func (a *App) riskScenarioHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	portfolioID, ok := a.portfolioIDForRequest(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "portfolio not found"})
		return
	}
	var request domain.ScenarioRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	impact, err := a.service.Scenario(portfolioID, request)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, impact)
}

func (a *App) riskAlertRulesHandler(w http.ResponseWriter, r *http.Request) {
	portfolioID, ok := a.portfolioIDForRequest(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "portfolio not found"})
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, a.service.AlertRules(portfolioID))
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var request struct {
		Type      string   `json:"type"`
		Threshold float64  `json:"threshold"`
		Severity  string   `json:"severity"`
		Channels  []string `json:"channels"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	actor, _ := userIDFromRequest(r)
	rule, err := a.service.CreateAlertRule(portfolioID, request.Type, request.Threshold, request.Severity, actor, request.Channels)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

func (a *App) evaluateAlertsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	portfolioID, ok := a.portfolioIDForRequest(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "portfolio not found"})
		return
	}
	actor, _ := userIDFromRequest(r)
	writeJSON(w, http.StatusOK, a.service.EvaluateAlerts(portfolioID, actor))
}

func (a *App) alertEventsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	portfolioID, ok := a.portfolioIDForRequest(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "portfolio not found"})
		return
	}
	writeJSON(w, http.StatusOK, a.service.AlertEvents(portfolioID))
}

func (a *App) auditLogHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	portfolioID, ok := a.portfolioIDForRequest(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "portfolio not found"})
		return
	}
	writeJSON(w, http.StatusOK, a.service.AuditEntries(portfolioID))
}

func (a *App) brokerSyncHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	portfolioID, ok := a.portfolioIDForRequest(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "portfolio not found"})
		return
	}
	actor, _ := userIDFromRequest(r)
	count, err := a.service.SyncBroker(portfolioID, a.brokerProvider, actor)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"synced": count})
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
		actor, _ := userIDFromRequest(r)
		limit, err := a.service.AddRiskLimitWithActor(portfolioID, req.Type, req.Value, actor)
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
