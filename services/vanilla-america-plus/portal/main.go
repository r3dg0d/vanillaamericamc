package main

import (
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed static/*
var staticFiles embed.FS

type App struct {
	store         *Store
	logger        *slog.Logger
	bind          string
	serverAddress string
	serverLog     string
	rconPassword  string
	systemctl     string
	serviceName   string
	loginLimiter  *loginLimiter
}

type loginAttempt struct {
	count      int
	windowEnds time.Time
}

type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string]loginAttempt
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "reset-admin" {
		if err := resetAdministratorCLI(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "reset-admin failed:", err)
			os.Exit(1)
		}
		return
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	dbPath := env("VA_DATABASE", "/var/lib/vanilla-america-plus/integration/va-plus.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		logger.Error("database_open_failed", "error_type", fmt.Sprintf("%T", err))
		os.Exit(1)
	}
	defer store.Close()

	password, err := readSecretFile(env("VA_BOOTSTRAP_PASSWORD_FILE",
		"/var/lib/vanilla-america-plus/credentials/portal-admin-password"))
	if err != nil {
		logger.Error("bootstrap_secret_unavailable", "error_type", fmt.Sprintf("%T", err))
		os.Exit(1)
	}
	if err := store.BootstrapAdministrator("admin", password); err != nil {
		logger.Error("bootstrap_admin_failed", "error_type", fmt.Sprintf("%T", err))
		os.Exit(1)
	}

	rconPassword, err := readSecretFile(env("VA_RCON_PASSWORD_FILE",
		"/var/lib/vanilla-america-plus/credentials/rcon-password"))
	if err != nil {
		logger.Warn("rcon_secret_unavailable", "error_type", fmt.Sprintf("%T", err))
	}

	app := &App{
		store:         store,
		logger:        logger,
		bind:          env("VA_BIND", "127.0.0.1:18080"),
		serverAddress: env("VA_SERVER_ADDRESS", "127.0.0.1:25565"),
		serverLog:     env("VA_SERVER_LOG", "/var/lib/vanilla-america-plus/server/logs/latest.log"),
		rconPassword:  rconPassword,
		systemctl:     env("VA_SYSTEMCTL", "/run/current-system/sw/bin/systemctl"),
		serviceName:   env("VA_SERVICE_NAME", "vanilla-america-plus.service"),
		loginLimiter:  &loginLimiter{attempts: make(map[string]loginAttempt)},
	}

	server := &http.Server{
		Addr:              app.bind,
		Handler:           app.securityHeaders(app.routes()),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}
	logger.Info("portal_starting", "bind", app.bind)
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		logger.Error("portal_stopped", "error_type", fmt.Sprintf("%T", err))
		os.Exit(1)
	}
}

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/session", a.handleSession)
	mux.HandleFunc("POST /api/auth/login", a.handleLogin)
	mux.Handle("POST /api/auth/logout", a.withSession("player", http.HandlerFunc(a.handleLogout)))
	mux.Handle("GET /api/me", a.withSession("player", http.HandlerFunc(a.handleMe)))
	mux.HandleFunc("GET /api/status", a.handleStatus)
	mux.HandleFunc("GET /api/catalog", a.handleCatalog)
	mux.HandleFunc("POST /api/orders", a.handleCreateOrder)

	mux.Handle("GET /api/staff/reports", a.withSession("moderator", http.HandlerFunc(a.handleReports)))
	mux.Handle("POST /api/staff/reports/{id}", a.withSession("moderator", http.HandlerFunc(a.handleReportAction)))
	mux.Handle("POST /api/staff/moderation", a.withSession("moderator", http.HandlerFunc(a.handleModeration)))

	mux.Handle("GET /api/admin/audit", a.withSession("administrator", http.HandlerFunc(a.handleAudit)))
	mux.Handle("GET /api/admin/orders", a.withSession("administrator", http.HandlerFunc(a.handleOrders)))
	mux.Handle("POST /api/admin/users", a.withSession("administrator", http.HandlerFunc(a.handleCreateUser)))
	mux.Handle("POST /api/admin/orders/{id}/fulfill", a.withSession("administrator", http.HandlerFunc(a.handleFulfillOrder)))
	mux.Handle("POST /api/admin/server", a.withSession("administrator", http.HandlerFunc(a.handleServerAction)))
	mux.Handle("GET /api/admin/console", a.withSession("administrator", http.HandlerFunc(a.handleConsoleRead)))
	mux.Handle("POST /api/admin/console", a.withSession("administrator", http.HandlerFunc(a.handleConsoleCommand)))

	content, _ := fs.Sub(staticFiles, "static")
	mux.Handle("/", spaHandler(http.FS(content)))
	return mux
}

func (a *App) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.Header().Set("X-Frame-Options", "DENY")
		response.Header().Set("Referrer-Policy", "no-referrer")
		response.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		response.Header().Set("Content-Security-Policy",
			"default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		response.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(response, request)
	})
}

func (a *App) handleSession(response http.ResponseWriter, request *http.Request) {
	csrf, err := randomToken(24)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "session unavailable")
		return
	}
	http.SetCookie(response, csrfCookie(csrf))
	writeJSON(response, http.StatusOK, map[string]string{"csrf": csrf})
}

func (a *App) handleLogin(response http.ResponseWriter, request *http.Request) {
	if !validPublicCSRF(request) {
		writeError(response, http.StatusForbidden, "invalid CSRF token")
		return
	}
	ip, _, _ := net.SplitHostPort(request.RemoteAddr)
	if !a.loginLimiter.allow(ip) {
		writeError(response, http.StatusTooManyRequests, "too many login attempts")
		return
	}
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(response, request, &input); err != nil {
		return
	}
	user, err := a.store.Authenticate(input.Username, input.Password)
	if err != nil {
		time.Sleep(350 * time.Millisecond)
		_ = a.store.Audit(input.Username, "auth.login", "portal", "failed")
		writeError(response, http.StatusUnauthorized, "invalid credentials")
		return
	}
	a.loginLimiter.success(ip)
	token, session, err := a.store.NewSession(user)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "login unavailable")
		return
	}
	_ = a.store.Audit(user.Username, "auth.login", "portal", "success")
	http.SetCookie(response, &http.Cookie{
		Name: "va_session", Value: token, Path: "/", MaxAge: 12 * 60 * 60,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode,
	})
	http.SetCookie(response, csrfCookie(session.CSRF))
	writeJSON(response, http.StatusOK, map[string]any{
		"id": session.ID, "username": session.Username, "role": session.Role, "csrf": session.CSRF,
	})
}

func (a *App) handleLogout(response http.ResponseWriter, request *http.Request) {
	cookie, _ := request.Cookie("va_session")
	session := sessionFromContext(request.Context())
	if cookie != nil {
		a.store.DeleteSession(cookie.Value)
	}
	_ = a.store.Audit(session.Username, "auth.logout", "portal", "success")
	http.SetCookie(response, &http.Cookie{
		Name: "va_session", Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode,
	})
	response.WriteHeader(http.StatusNoContent)
}

func (a *App) handleMe(response http.ResponseWriter, request *http.Request) {
	writeJSON(response, http.StatusOK, sessionFromContext(request.Context()).User)
}

func (a *App) handleStatus(response http.ResponseWriter, request *http.Request) {
	status := pingMinecraft(a.serverAddress, 1500*time.Millisecond)
	writeJSON(response, http.StatusOK, status)
}

func (a *App) handleCatalog(response http.ResponseWriter, request *http.Request) {
	items := make([]CatalogItem, 0, len(catalog))
	for _, item := range catalog {
		items = append(items, item)
	}
	writeJSON(response, http.StatusOK, map[string]any{"mode": "mock", "items": items})
}

func (a *App) handleCreateOrder(response http.ResponseWriter, request *http.Request) {
	if !validPublicCSRF(request) {
		writeError(response, http.StatusForbidden, "invalid CSRF token")
		return
	}
	var input struct {
		PlayerUUID string         `json:"player_uuid"`
		Items      map[string]int `json:"items"`
	}
	if err := decodeJSON(response, request, &input); err != nil {
		return
	}
	id, total, err := a.store.CreateMockOrder(input.PlayerUUID, input.Items)
	if err != nil {
		writeError(response, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(response, http.StatusCreated, map[string]any{
		"id": id, "total_cents": total, "state": "mock_recorded",
		"message": "Test order recorded. No payment was attempted.",
	})
}

func (a *App) handleReports(response http.ResponseWriter, request *http.Request) {
	reports, err := a.store.ListReports(100)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "reports unavailable")
		return
	}
	writeJSON(response, http.StatusOK, reports)
}

func (a *App) handleReportAction(response http.ResponseWriter, request *http.Request) {
	session := sessionFromContext(request.Context())
	var input struct {
		Action string `json:"action"`
		Note   string `json:"note"`
	}
	if err := decodeJSON(response, request, &input); err != nil {
		return
	}
	if len(input.Note) > 240 {
		writeError(response, http.StatusBadRequest, "note is too long")
		return
	}
	action := map[string]string{"resolve": "resolved", "dismiss": "dismissed"}[input.Action]
	if action == "" {
		writeError(response, http.StatusBadRequest, "unsupported action")
		return
	}
	if err := a.store.ResolveReport(session.Username, request.PathValue("id"), action, input.Note); err != nil {
		writeError(response, http.StatusConflict, err.Error())
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func (a *App) handleModeration(response http.ResponseWriter, request *http.Request) {
	session := sessionFromContext(request.Context())
	var input struct {
		Action          string `json:"action"`
		Target          string `json:"target"`
		Reason          string `json:"reason"`
		DurationSeconds int    `json:"duration_seconds"`
	}
	if err := decodeJSON(response, request, &input); err != nil {
		return
	}
	if !validPlayerName(input.Target) || len(input.Reason) < 4 || len(input.Reason) > 240 {
		writeError(response, http.StatusBadRequest, "invalid target or reason")
		return
	}
	id, err := a.store.QueueModeration(
		session.Username, input.Action, input.Target, input.Reason, input.DurationSeconds,
	)
	if err != nil {
		writeError(response, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(response, http.StatusAccepted, map[string]string{"id": id, "state": "pending"})
}

func (a *App) handleAudit(response http.ResponseWriter, request *http.Request) {
	entries, err := a.store.AuditEntries(200)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "audit unavailable")
		return
	}
	writeJSON(response, http.StatusOK, entries)
}

func (a *App) handleOrders(response http.ResponseWriter, request *http.Request) {
	orders, err := a.store.ListOrders(100)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "orders unavailable")
		return
	}
	writeJSON(response, http.StatusOK, orders)
}

func (a *App) handleCreateUser(response http.ResponseWriter, request *http.Request) {
	session := sessionFromContext(request.Context())
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := decodeJSON(response, request, &input); err != nil {
		return
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_.-]{3,32}$`).MatchString(input.Username) {
		writeError(response, http.StatusBadRequest, "invalid username")
		return
	}
	if err := a.store.CreateUser(session.Username, input.Username, input.Password, input.Role); err != nil {
		writeError(response, http.StatusBadRequest, "unable to create user")
		return
	}
	response.WriteHeader(http.StatusCreated)
}

func (a *App) handleFulfillOrder(response http.ResponseWriter, request *http.Request) {
	session := sessionFromContext(request.Context())
	if err := a.store.FulfillMockOrder(session.Username, request.PathValue("id")); err != nil {
		writeError(response, http.StatusConflict, err.Error())
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func (a *App) handleServerAction(response http.ResponseWriter, request *http.Request) {
	session := sessionFromContext(request.Context())
	var input struct {
		Action string `json:"action"`
	}
	if err := decodeJSON(response, request, &input); err != nil {
		return
	}
	allowed := map[string]string{"start": "start", "stop": "stop", "restart": "restart"}
	verb := allowed[input.Action]
	if verb == "" {
		writeError(response, http.StatusBadRequest, "unsupported lifecycle action")
		return
	}
	command := exec.Command("/run/wrappers/bin/sudo", "-n", a.systemctl, verb, a.serviceName)
	output, err := command.CombinedOutput()
	outcome := "success"
	if err != nil {
		outcome = "failed"
		a.logger.Warn("server_action_failed", "actor", session.Username, "action", verb, "output_bytes", len(output))
	}
	_ = a.store.Audit(session.Username, "server."+verb, a.serviceName, outcome)
	if err != nil {
		writeError(response, http.StatusBadGateway, "server action failed")
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func (a *App) handleConsoleRead(response http.ResponseWriter, request *http.Request) {
	lines := 160
	if parsed, err := strconv.Atoi(request.URL.Query().Get("lines")); err == nil {
		lines = min(max(parsed, 10), 300)
	}
	content, err := tailFile(a.serverLog, lines, 512<<10)
	if err != nil {
		writeJSON(response, http.StatusOK, map[string]any{"available": false, "lines": []string{}})
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"available": true, "lines": content})
}

func (a *App) handleConsoleCommand(response http.ResponseWriter, request *http.Request) {
	session := sessionFromContext(request.Context())
	var input struct {
		Command string `json:"command"`
	}
	if err := decodeJSON(response, request, &input); err != nil {
		return
	}
	command := strings.TrimSpace(input.Command)
	if !allowedConsoleCommand(command) {
		_ = a.store.Audit(session.Username, "console.command", "rejected", "not_allowlisted")
		writeError(response, http.StatusForbidden, "command is not allowlisted")
		return
	}
	result, err := rconCommand("127.0.0.1:25575", a.rconPassword, command, 3*time.Second)
	outcome := "success"
	if err != nil {
		outcome = "failed"
	}
	_ = a.store.Audit(session.Username, "console.command", strings.Fields(command)[0], outcome)
	if err != nil {
		writeError(response, http.StatusBadGateway, "console unavailable")
		return
	}
	writeJSON(response, http.StatusOK, map[string]string{"output": result})
}

func (a *App) withSession(minimumRole string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		cookie, err := request.Cookie("va_session")
		if err != nil {
			writeError(response, http.StatusUnauthorized, "authentication required")
			return
		}
		session, err := a.store.Session(cookie.Value)
		if err != nil {
			writeError(response, http.StatusUnauthorized, "authentication required")
			return
		}
		if roleRank(session.Role) < roleRank(minimumRole) {
			_ = a.store.Audit(session.Username, "authorization.denied", request.URL.Path, "forbidden")
			writeError(response, http.StatusForbidden, "insufficient role")
			return
		}
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			csrfCookieValue, err := request.Cookie("va_csrf")
			header := request.Header.Get("X-CSRF-Token")
			if err != nil || subtle.ConstantTimeCompare([]byte(header), []byte(session.CSRF)) != 1 ||
				subtle.ConstantTimeCompare([]byte(header), []byte(csrfCookieValue.Value)) != 1 {
				writeError(response, http.StatusForbidden, "invalid CSRF token")
				return
			}
		}
		next.ServeHTTP(response, request.WithContext(withSession(request.Context(), session)))
	})
}

func (limiter *loginLimiter) allow(ip string) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	now := time.Now()
	attempt := limiter.attempts[ip]
	if now.After(attempt.windowEnds) {
		attempt = loginAttempt{windowEnds: now.Add(15 * time.Minute)}
	}
	attempt.count++
	limiter.attempts[ip] = attempt
	return attempt.count <= 5
}

func (limiter *loginLimiter) success(ip string) {
	limiter.mu.Lock()
	delete(limiter.attempts, ip)
	limiter.mu.Unlock()
}

func decodeJSON(response http.ResponseWriter, request *http.Request, destination any) error {
	request.Body = http.MaxBytesReader(response, request.Body, 32<<10)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeError(response, http.StatusBadRequest, "invalid request body")
		return err
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		writeError(response, http.StatusBadRequest, "request body must contain one JSON value")
		return errors.New("extra JSON content")
	}
	return nil
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

func writeError(response http.ResponseWriter, status int, message string) {
	writeJSON(response, status, map[string]string{"error": message})
}

func roleRank(role string) int {
	return map[string]int{"player": 1, "moderator": 2, "administrator": 3}[role]
}

func csrfCookie(value string) *http.Cookie {
	return &http.Cookie{
		Name: "va_csrf", Value: value, Path: "/", MaxAge: 12 * 60 * 60,
		HttpOnly: false, Secure: true, SameSite: http.SameSiteStrictMode,
	}
}

func validPublicCSRF(request *http.Request) bool {
	cookie, err := request.Cookie("va_csrf")
	if err != nil {
		return false
	}
	header := request.Header.Get("X-CSRF-Token")
	return header != "" && subtle.ConstantTimeCompare([]byte(header), []byte(cookie.Value)) == 1
}

func allowedConsoleCommand(command string) bool {
	if len(command) < 1 || len(command) > 240 || strings.ContainsAny(command, "\r\n\x00") {
		return false
	}
	lower := strings.ToLower(command)
	for _, prefix := range []string{
		"list", "save-all", "say ", "whitelist add ", "whitelist remove ", "whitelist list",
		"kick ", "ban ", "pardon ", "weather ", "time set ", "vaplus ", "lp user ",
	} {
		if lower == prefix || strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func validPlayerName(value string) bool {
	return regexp.MustCompile(`^[.+_A-Za-z0-9 -]{1,32}$`).MatchString(value)
}

func readSecretFile(path string) (string, error) {
	value, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	secret := strings.TrimSpace(string(value))
	if len(secret) < 14 || len(secret) > 256 {
		return "", errors.New("secret length is invalid")
	}
	return secret, nil
}

func tailFile(path string, lineLimit, byteLimit int) ([]string, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	start := max(int64(0), info.Size()-int64(byteLimit))
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	content, err := io.ReadAll(io.LimitReader(file, int64(byteLimit)))
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
	if start > 0 && len(lines) > 0 {
		lines = lines[1:]
	}
	if len(lines) > lineLimit {
		lines = lines[len(lines)-lineLimit:]
	}
	return lines, nil
}

func spaHandler(files http.FileSystem) http.Handler {
	static := http.FileServer(files)
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		path := strings.TrimPrefix(filepath.Clean(request.URL.Path), "/")
		if path == "." {
			path = "index.html"
		}
		if file, err := files.Open(path); err == nil {
			file.Close()
			static.ServeHTTP(response, request)
			return
		}
		request.URL.Path = "/"
		static.ServeHTTP(response, request)
	})
}

func resetAdministratorCLI(arguments []string) error {
	if len(arguments) != 3 {
		return errors.New("usage: reset-admin <database> <username> <password-file>")
	}
	password, err := readSecretFile(arguments[2])
	if err != nil {
		return err
	}
	store, err := OpenStore(arguments[0])
	if err != nil {
		return err
	}
	defer store.Close()
	return store.ResetAdministrator(arguments[1], password)
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
