package lighter

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTokenFresh(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	exp := now.Add(7 * time.Hour)
	if !tokenFresh(now, exp) {
		t.Fatal("fresh token should still be usable")
	}
	if tokenFresh(exp.Add(-5*time.Minute), exp) {
		t.Fatal("token inside refresh skew should renew")
	}
	if tokenFresh(exp.Add(time.Second), exp) {
		t.Fatal("expired token is not fresh")
	}
}

func TestActiveOrdersDoesNotFallbackOn401(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if strings.Contains(r.URL.Path, "accountActiveOrders") {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"code":20013,"message":"invalid auth: expired token"}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, "404 page not found")
	}))
	defer srv.Close()

	_, err := NewHTTP(srv.URL).ActiveOrders(context.Background(), 1, 0)
	if err == nil {
		t.Fatal("want 401")
	}
	if !authHTTPError(err) {
		t.Fatalf("want auth error, got %v", err)
	}
	if len(paths) != 1 || !strings.Contains(paths[0], "accountActiveOrders") {
		t.Fatalf("must not hit /orders fallback, paths=%v", paths)
	}
	if strings.Contains(err.Error(), "404") {
		t.Fatalf("401 must not glue 404 fallback: %v", err)
	}
}
