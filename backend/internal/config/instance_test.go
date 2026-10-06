package config

import "testing"

func TestSanitizeInstanceID(t *testing.T) {
	if got := sanitizeInstanceID("Main-1"); got != "main-1" {
		t.Fatalf("got %q", got)
	}
	if got := sanitizeInstanceID("@@@"); got != "a" {
		t.Fatalf("empty fallback %q", got)
	}
	cfg := &Config{InstanceID: "b"}
	if cfg.SessionCookieName() != "donchian_session_b" {
		t.Fatalf("cookie %q", cfg.SessionCookieName())
	}
}
