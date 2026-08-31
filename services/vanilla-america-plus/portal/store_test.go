package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

const testPassword = "correct-horse-battery-staple"

func testStore(t *testing.T) *Store {
	t.Helper()
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestPasswordAndSessionLifecycle(t *testing.T) {
	store := testStore(t)
	if err := store.BootstrapAdministrator("admin", testPassword); err != nil {
		t.Fatal(err)
	}
	user, err := store.Authenticate("admin", testPassword)
	if err != nil || user.Role != "administrator" {
		t.Fatalf("administrator authentication failed: %v", err)
	}
	if _, err := store.Authenticate("admin", "not-the-password"); err == nil {
		t.Fatal("wrong password was accepted")
	}
	token, session, err := store.NewSession(user)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Session(token)
	if err != nil || loaded.CSRF != session.CSRF {
		t.Fatalf("session lookup failed: %v", err)
	}
	store.DeleteSession(token)
	if _, err := store.Session(token); err == nil {
		t.Fatal("deleted session remained valid")
	}
}

func TestMockFulfillmentIsIdempotentAndAllowlisted(t *testing.T) {
	store := testStore(t)
	orderID, total, err := store.CreateMockOrder(
		"123e4567-e89b-12d3-a456-426614174000",
		map[string]int{"supporter_badge": 2},
	)
	if err != nil || total != 1000 {
		t.Fatalf("create order: total=%d err=%v", total, err)
	}
	if err := store.FulfillMockOrder("admin", orderID); err != nil {
		t.Fatal(err)
	}
	if err := store.FulfillMockOrder("admin", orderID); err != nil {
		t.Fatalf("idempotent retry failed: %v", err)
	}
	var count int
	if err := store.db.QueryRow(
		"SELECT COUNT(*) FROM entitlements WHERE order_id=?", orderID,
	).Scan(&count); err != nil || count != 1 {
		t.Fatalf("expected one entitlement, count=%d err=%v", count, err)
	}
	if _, _, err := store.CreateMockOrder(
		"123e4567-e89b-12d3-a456-426614174000",
		map[string]int{"operator": 1},
	); err == nil {
		t.Fatal("non-allowlisted entitlement was accepted")
	}
}

func TestModeratorCannotUseAdministratorRoute(t *testing.T) {
	store := testStore(t)
	if err := store.BootstrapAdministrator("admin", testPassword); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateUser("admin", "mod", testPassword, "moderator"); err != nil {
		t.Fatal(err)
	}
	user, err := store.Authenticate("mod", testPassword)
	if err != nil {
		t.Fatal(err)
	}
	token, session, err := store.NewSession(user)
	if err != nil {
		t.Fatal(err)
	}
	app := &App{
		store: store, logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		loginLimiter: &loginLimiter{attempts: make(map[string]loginAttempt)},
	}
	handler := app.withSession("administrator", http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodPost, "/api/admin/server", nil)
	request.AddCookie(&http.Cookie{Name: "va_session", Value: token})
	request.AddCookie(&http.Cookie{Name: "va_csrf", Value: session.CSRF})
	request.Header.Set("X-CSRF-Token", session.CSRF)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("moderator received status %d, want 403", response.Code)
	}
}

func TestAuditRowsAreImmutable(t *testing.T) {
	store := testStore(t)
	if err := store.Audit("admin", "test", "record", "success"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec("DELETE FROM staff_audit"); err == nil {
		t.Fatal("audit row deletion was permitted")
	}
}
