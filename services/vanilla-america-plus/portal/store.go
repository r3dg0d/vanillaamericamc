package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type Session struct {
	User
	CSRF      string
	ExpiresAt time.Time
}

type Report struct {
	ID             string  `json:"id"`
	ReporterName   string  `json:"reporter_name"`
	TargetName     string  `json:"target_name"`
	Reason         string  `json:"reason"`
	Status         string  `json:"status"`
	CreatedAt      string  `json:"created_at"`
	ResolvedBy     *string `json:"resolved_by,omitempty"`
	ResolutionNote *string `json:"resolution_note,omitempty"`
}

type AuditEntry struct {
	ID        int64  `json:"id"`
	Actor     string `json:"actor"`
	Action    string `json:"action"`
	Target    string `json:"target"`
	Outcome   string `json:"outcome"`
	CreatedAt string `json:"created_at"`
}

type Order struct {
	ID          string  `json:"id"`
	PlayerUUID  string  `json:"player_uuid"`
	State       string  `json:"state"`
	TotalCents  int     `json:"total_cents"`
	CreatedAt   string  `json:"created_at"`
	FulfilledAt *string `json:"fulfilled_at,omitempty"`
}

type CatalogItem struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceCents  int    `json:"price_cents"`
	Category    string `json:"category"`
}

var catalog = map[string]CatalogItem{
	"supporter_badge": {
		Code: "supporter_badge", Name: "Founders Supporter Badge",
		Description: "A cosmetic chat badge permission. No gameplay advantage.", PriceCents: 500, Category: "Cosmetic",
	},
	"liberty_bell": {
		Code: "liberty_bell", Name: "Liberty Bell Celebration Access",
		Description: "Permission to request the server-wide cosmetic celebration, subject to cooldown.", PriceCents: 300, Category: "Community",
	},
}

func OpenStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS portal_users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL CHECK(role IN ('player','moderator','administrator')),
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS portal_sessions (
			token_hash TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES portal_users(id) ON DELETE CASCADE,
			csrf TEXT NOT NULL,
			expires_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS reports (
			id TEXT PRIMARY KEY,
			reporter_uuid TEXT NOT NULL,
			reporter_name TEXT NOT NULL,
			target_name TEXT NOT NULL,
			reason TEXT NOT NULL,
			status TEXT NOT NULL CHECK(status IN ('open','resolved','dismissed')),
			created_at TEXT NOT NULL,
			resolved_by TEXT,
			resolution_note TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS staff_audit (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			actor TEXT NOT NULL,
			action TEXT NOT NULL,
			target TEXT NOT NULL,
			outcome TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TRIGGER IF NOT EXISTS staff_audit_no_update
		 BEFORE UPDATE ON staff_audit BEGIN SELECT RAISE(ABORT, 'staff_audit is immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS staff_audit_no_delete
		 BEFORE DELETE ON staff_audit BEGIN SELECT RAISE(ABORT, 'staff_audit is immutable'); END`,
		`CREATE TABLE IF NOT EXISTS orders (
			id TEXT PRIMARY KEY,
			player_uuid TEXT NOT NULL,
			state TEXT NOT NULL CHECK(state IN ('mock_recorded','mock_fulfilled','cancelled')),
			total_cents INTEGER NOT NULL,
			created_at TEXT NOT NULL,
			fulfilled_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS order_items (
			order_id TEXT NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
			entitlement_code TEXT NOT NULL,
			quantity INTEGER NOT NULL CHECK(quantity BETWEEN 1 AND 10),
			unit_cents INTEGER NOT NULL,
			PRIMARY KEY(order_id, entitlement_code)
		)`,
		`CREATE TABLE IF NOT EXISTS entitlements (
			order_id TEXT NOT NULL,
			player_uuid TEXT NOT NULL,
			entitlement_code TEXT NOT NULL,
			state TEXT NOT NULL CHECK(state IN ('pending','processing','applied','rejected')),
			detail TEXT,
			created_at TEXT NOT NULL,
			applied_at TEXT,
			PRIMARY KEY(order_id, entitlement_code)
		)`,
		`CREATE TABLE IF NOT EXISTS moderation_actions (
			id TEXT PRIMARY KEY,
			actor TEXT NOT NULL,
			action TEXT NOT NULL CHECK(action IN ('warn','mute','kick','tempban')),
			target_name TEXT NOT NULL,
			reason TEXT NOT NULL,
			duration_seconds INTEGER NOT NULL,
			state TEXT NOT NULL CHECK(state IN ('pending','processing','applied','rejected')),
			detail TEXT,
			created_at TEXT NOT NULL,
			applied_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS player_mutes (
			player_uuid TEXT PRIMARY KEY,
			player_name TEXT NOT NULL,
			reason TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			actor TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS reports_status_created ON reports(status, created_at)`,
		`CREATE INDEX IF NOT EXISTS moderation_state_created ON moderation_actions(state, created_at)`,
	}
	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return fmt.Errorf("database migration: %w", err)
		}
	}
	return nil
}

func (s *Store) BootstrapAdministrator(username, password string) error {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM portal_users").Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return nil
	}
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		"INSERT INTO portal_users(username,password_hash,role,created_at) VALUES(?,?,?,?)",
		username, hash, "administrator", time.Now().UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (s *Store) ResetAdministrator(username, password string) error {
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	result, err := tx.Exec("UPDATE portal_users SET password_hash=?, role='administrator' WHERE username=?", hash, username)
	if err != nil {
		tx.Rollback()
		return err
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		_, err = tx.Exec(
			"INSERT INTO portal_users(username,password_hash,role,created_at) VALUES(?,?,?,?)",
			username, hash, "administrator", time.Now().UTC().Format(time.RFC3339Nano),
		)
	}
	if err == nil {
		_, err = tx.Exec("DELETE FROM portal_sessions")
	}
	if err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) Authenticate(username, password string) (User, error) {
	var user User
	var hash string
	err := s.db.QueryRow(
		"SELECT id,username,role,password_hash FROM portal_users WHERE username=?", username,
	).Scan(&user.ID, &user.Username, &user.Role, &hash)
	if err != nil || !verifyPassword(password, hash) {
		return User{}, errors.New("invalid credentials")
	}
	return user, nil
}

func (s *Store) NewSession(user User) (token string, session Session, err error) {
	token, err = randomToken(32)
	if err != nil {
		return "", Session{}, err
	}
	csrf, err := randomToken(24)
	if err != nil {
		return "", Session{}, err
	}
	expires := time.Now().UTC().Add(12 * time.Hour)
	sum := sha256.Sum256([]byte(token))
	_, err = s.db.Exec(
		"INSERT INTO portal_sessions(token_hash,user_id,csrf,expires_at) VALUES(?,?,?,?)",
		hex.EncodeToString(sum[:]), user.ID, csrf, expires.Format(time.RFC3339Nano),
	)
	return token, Session{User: user, CSRF: csrf, ExpiresAt: expires}, err
}

func (s *Store) Session(token string) (Session, error) {
	sum := sha256.Sum256([]byte(token))
	var session Session
	var expires string
	err := s.db.QueryRow(`
		SELECT u.id,u.username,u.role,s.csrf,s.expires_at
		FROM portal_sessions s JOIN portal_users u ON u.id=s.user_id
		WHERE s.token_hash=?
	`, hex.EncodeToString(sum[:])).Scan(
		&session.ID, &session.Username, &session.Role, &session.CSRF, &expires,
	)
	if err != nil {
		return Session{}, err
	}
	session.ExpiresAt, err = time.Parse(time.RFC3339Nano, expires)
	if err != nil || time.Now().UTC().After(session.ExpiresAt) {
		return Session{}, errors.New("session expired")
	}
	return session, nil
}

func (s *Store) DeleteSession(token string) {
	sum := sha256.Sum256([]byte(token))
	_, _ = s.db.Exec("DELETE FROM portal_sessions WHERE token_hash=?", hex.EncodeToString(sum[:]))
}

func (s *Store) CreateUser(actor, username, password, role string) error {
	if role != "moderator" && role != "administrator" && role != "player" {
		return errors.New("invalid role")
	}
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		"INSERT INTO portal_users(username,password_hash,role,created_at) VALUES(?,?,?,?)",
		username, hash, role, time.Now().UTC().Format(time.RFC3339Nano),
	)
	outcome := "success"
	if err != nil {
		outcome = "failed"
	}
	_ = s.Audit(actor, "user.create."+role, username, outcome)
	return err
}

func (s *Store) ListReports(limit int) ([]Report, error) {
	rows, err := s.db.Query(`
		SELECT id,reporter_name,target_name,reason,status,created_at,resolved_by,resolution_note
		FROM reports ORDER BY CASE status WHEN 'open' THEN 0 ELSE 1 END, created_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reports []Report
	for rows.Next() {
		var report Report
		if err := rows.Scan(
			&report.ID, &report.ReporterName, &report.TargetName, &report.Reason,
			&report.Status, &report.CreatedAt, &report.ResolvedBy, &report.ResolutionNote,
		); err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	return reports, rows.Err()
}

func (s *Store) ResolveReport(actor, reportID, action, note string) error {
	if action != "resolved" && action != "dismissed" {
		return errors.New("invalid report action")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	result, err := tx.Exec(
		"UPDATE reports SET status=?,resolved_by=?,resolution_note=? WHERE id=? AND status='open'",
		action, actor, note, reportID,
	)
	if err != nil {
		tx.Rollback()
		return err
	}
	changed, _ := result.RowsAffected()
	outcome := "not_open"
	if changed == 1 {
		outcome = "success"
	}
	_, err = tx.Exec(
		"INSERT INTO staff_audit(actor,action,target,outcome,created_at) VALUES(?,?,?,?,?)",
		actor, "report."+action, reportID, outcome, time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if changed != 1 {
		return errors.New("report is not open")
	}
	return nil
}

func (s *Store) QueueModeration(actor, action, target, reason string, durationSeconds int) (string, error) {
	switch action {
	case "warn", "kick":
		durationSeconds = 0
	case "mute", "tempban":
		if durationSeconds < 60 || durationSeconds > 30*24*60*60 {
			return "", errors.New("duration must be between 60 seconds and 30 days")
		}
	default:
		return "", errors.New("unsupported moderation action")
	}
	id, err := randomToken(18)
	if err != nil {
		return "", err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(`
		INSERT INTO moderation_actions(id,actor,action,target_name,reason,duration_seconds,state,created_at)
		VALUES(?,?,?,?,?,?,'pending',?)
	`, id, actor, action, target, reason, durationSeconds, now)
	if err == nil {
		_, err = tx.Exec(
			"INSERT INTO staff_audit(actor,action,target,outcome,created_at) VALUES(?,?,?,?,?)",
			actor, "moderation."+action, target, "queued", now,
		)
	}
	if err != nil {
		tx.Rollback()
		return "", err
	}
	return id, tx.Commit()
}

func (s *Store) CreateMockOrder(playerUUID string, quantities map[string]int) (string, int, error) {
	if !validUUID(playerUUID) {
		return "", 0, errors.New("player UUID is invalid")
	}
	if len(quantities) == 0 || len(quantities) > 8 {
		return "", 0, errors.New("cart is empty or too large")
	}
	total := 0
	for code, quantity := range quantities {
		item, ok := catalog[code]
		if !ok || quantity < 1 || quantity > 10 {
			return "", 0, errors.New("cart contains an unsupported item")
		}
		total += item.PriceCents * quantity
	}
	orderID, err := randomToken(18)
	if err != nil {
		return "", 0, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return "", 0, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.Exec(
		"INSERT INTO orders(id,player_uuid,state,total_cents,created_at) VALUES(?,?,'mock_recorded',?,?)",
		orderID, strings.ToLower(playerUUID), total, now,
	)
	if err == nil {
		for code, quantity := range quantities {
			item := catalog[code]
			_, err = tx.Exec(
				"INSERT INTO order_items(order_id,entitlement_code,quantity,unit_cents) VALUES(?,?,?,?)",
				orderID, code, quantity, item.PriceCents,
			)
			if err != nil {
				break
			}
		}
	}
	if err != nil {
		tx.Rollback()
		return "", 0, err
	}
	return orderID, total, tx.Commit()
}

func (s *Store) FulfillMockOrder(actor, orderID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	var playerUUID, state string
	err = tx.QueryRow("SELECT player_uuid,state FROM orders WHERE id=?", orderID).Scan(&playerUUID, &state)
	if err != nil {
		tx.Rollback()
		return err
	}
	if state == "mock_fulfilled" {
		tx.Rollback()
		return nil
	}
	if state != "mock_recorded" {
		tx.Rollback()
		return errors.New("order cannot be fulfilled")
	}
	rows, err := tx.Query("SELECT entitlement_code FROM order_items WHERE order_id=?", orderID)
	if err != nil {
		tx.Rollback()
		return err
	}
	var codes []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			rows.Close()
			tx.Rollback()
			return err
		}
		codes = append(codes, code)
	}
	rows.Close()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, code := range codes {
		if _, ok := catalog[code]; !ok {
			tx.Rollback()
			return errors.New("order contains non-allowlisted entitlement")
		}
		_, err = tx.Exec(`
			INSERT INTO entitlements(order_id,player_uuid,entitlement_code,state,created_at)
			VALUES(?,?,?,'pending',?)
			ON CONFLICT(order_id,entitlement_code) DO NOTHING
		`, orderID, playerUUID, code, now)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	_, err = tx.Exec("UPDATE orders SET state='mock_fulfilled',fulfilled_at=? WHERE id=?", now, orderID)
	if err == nil {
		_, err = tx.Exec(
			"INSERT INTO staff_audit(actor,action,target,outcome,created_at) VALUES(?,?,?,?,?)",
			actor, "shop.mock_fulfill", orderID, "success", now,
		)
	}
	if err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) ListOrders(limit int) ([]Order, error) {
	rows, err := s.db.Query(`
		SELECT id,player_uuid,state,total_cents,created_at,fulfilled_at
		FROM orders ORDER BY created_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	orders := make([]Order, 0)
	for rows.Next() {
		var order Order
		if err := rows.Scan(
			&order.ID, &order.PlayerUUID, &order.State, &order.TotalCents,
			&order.CreatedAt, &order.FulfilledAt,
		); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

func (s *Store) Audit(actor, action, target, outcome string) error {
	_, err := s.db.Exec(
		"INSERT INTO staff_audit(actor,action,target,outcome,created_at) VALUES(?,?,?,?,?)",
		actor, action, target, outcome, time.Now().UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (s *Store) AuditEntries(limit int) ([]AuditEntry, error) {
	rows, err := s.db.Query(
		"SELECT id,actor,action,target,outcome,created_at FROM staff_audit ORDER BY id DESC LIMIT ?", limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []AuditEntry
	for rows.Next() {
		var entry AuditEntry
		if err := rows.Scan(&entry.ID, &entry.Actor, &entry.Action, &entry.Target, &entry.Outcome, &entry.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func hashPassword(password string) (string, error) {
	if len(password) < 14 || len(password) > 256 {
		return "", errors.New("password must be 14 to 256 characters")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, 3, 64*1024, 4, 32)
	return fmt.Sprintf("$argon2id$v=19$m=65536,t=3,p=4$%s$%s",
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash)), nil
}

func verifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var memory uint32
	var iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(expected)))
	return subtleEqual(actual, expected)
}

func subtleEqual(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var difference byte
	for index := range left {
		difference |= left[index] ^ right[index]
	}
	return difference == 0
}

func randomToken(bytes int) (string, error) {
	value := make([]byte, bytes)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func validUUID(value string) bool {
	parts := strings.Split(value, "-")
	if len(parts) != 5 || len(value) != 36 {
		return false
	}
	lengths := []int{8, 4, 4, 4, 12}
	for index, part := range parts {
		if len(part) != lengths[index] {
			return false
		}
		if _, err := strconv.ParseUint(part, 16, 64); err != nil {
			return false
		}
	}
	return true
}

func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}
