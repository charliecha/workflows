package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/user/url-shortener/store"
)

// mockStore implements store.Storer for testing
type mockStore struct {
	shortenFn      func(url string, ttl int) (string, time.Time, error)
	resolveFn      func(code string) (string, error)
	recordAccessFn func(code, referer, ua string) error
	statsFn        func(code string) (*store.StatsResponse, error)
}

func (m *mockStore) Shorten(url string, ttl int) (string, time.Time, error) {
	return m.shortenFn(url, ttl)
}
func (m *mockStore) Resolve(code string) (string, error)     { return m.resolveFn(code) }
func (m *mockStore) RecordAccess(code, ref, ua string) error { return m.recordAccessFn(code, ref, ua) }
func (m *mockStore) Stats(code string) (*store.StatsResponse, error) {
	return m.statsFn(code)
}

func newTestRouter(s store.Storer) http.Handler {
	r := chi.NewRouter()
	r.Post("/shorten", ShortenHandler(s, "http://localhost:8080"))
	r.Get("/{code}/qr", QRHandler(s, "http://localhost:8080"))
	r.Get("/{code}", RedirectHandler(s))
	r.Get("/{code}/stats", StatsHandler(s))
	return r
}

func TestShortenHandler_OK(t *testing.T) {
	expiresAt := time.Now().UTC().Add(30 * 24 * time.Hour)
	s := &mockStore{
		shortenFn: func(url string, ttl int) (string, time.Time, error) {
			return "abc123", expiresAt, nil
		},
	}
	r := newTestRouter(s)
	req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(`{"url":"https://example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp shortenResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.ShortCode != "abc123" {
		t.Fatalf("expected short_code abc123, got %q", resp.ShortCode)
	}
	if resp.ShortURL != "http://localhost:8080/abc123" {
		t.Fatalf("unexpected short_url: %q", resp.ShortURL)
	}
	if resp.QRUrl != "http://localhost:8080/abc123/qr" {
		t.Fatalf("unexpected qr_url: %q", resp.QRUrl)
	}
	if resp.ExpiresAt.IsZero() {
		t.Fatal("expected non-zero expires_at")
	}
}

func TestShortenHandler_EmptyURL(t *testing.T) {
	s := &mockStore{}
	r := newTestRouter(s)
	req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(`{"url":""}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	var resp errorResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error != "url is required" {
		t.Fatalf("unexpected error: %q", resp.Error)
	}
}

func TestShortenHandler_InvalidURL(t *testing.T) {
	s := &mockStore{}
	r := newTestRouter(s)
	req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(`{"url":"ftp://bad.com"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestShortenHandler_InvalidBody(t *testing.T) {
	s := &mockStore{}
	r := newTestRouter(s)
	req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(`not json`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	var resp errorResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error != "invalid request body" {
		t.Fatalf("unexpected error: %q", resp.Error)
	}
}

func TestShortenHandler_NegativeTTL(t *testing.T) {
	s := &mockStore{}
	r := newTestRouter(s)
	req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(`{"url":"https://example.com","ttl":-1}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	var resp errorResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error != "ttl must be a non-negative integer" {
		t.Fatalf("unexpected error: %q", resp.Error)
	}
}

func TestShortenHandler_WithTTL(t *testing.T) {
	s := &mockStore{
		shortenFn: func(url string, ttl int) (string, time.Time, error) {
			expiresAt := time.Now().UTC().Add(time.Duration(ttl) * time.Second)
			return "abc123", expiresAt, nil
		},
	}
	r := newTestRouter(s)
	req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(`{"url":"https://example.com","ttl":3600}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp shortenResponse
	json.NewDecoder(w.Body).Decode(&resp)
	expected := time.Now().UTC().Add(3600 * time.Second)
	diff := resp.ExpiresAt.Sub(expected)
	if diff < -2*time.Second || diff > 2*time.Second {
		t.Fatalf("expires_at %v not ~1h from now", resp.ExpiresAt)
	}
}

func TestShortenHandler_DefaultTTL(t *testing.T) {
	s := &mockStore{
		shortenFn: func(url string, ttl int) (string, time.Time, error) {
			if ttl == 0 {
				ttl = 30 * 24 * 3600
			}
			return "abc123", time.Now().UTC().Add(time.Duration(ttl) * time.Second), nil
		},
	}
	r := newTestRouter(s)
	req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(`{"url":"https://example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp shortenResponse
	json.NewDecoder(w.Body).Decode(&resp)
	expected := time.Now().UTC().Add(30 * 24 * time.Hour)
	diff := resp.ExpiresAt.Sub(expected)
	if diff < -2*time.Second || diff > 2*time.Second {
		t.Fatalf("expires_at %v not ~30d from now", resp.ExpiresAt)
	}
}

func TestRedirectHandler_Found(t *testing.T) {
	s := &mockStore{
		resolveFn:      func(code string) (string, error) { return "https://example.com", nil },
		recordAccessFn: func(code, ref, ua string) error { return nil },
	}
	r := newTestRouter(s)
	req := httptest.NewRequest(http.MethodGet, "/abc123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
	if w.Header().Get("Location") != "https://example.com" {
		t.Fatalf("unexpected Location: %q", w.Header().Get("Location"))
	}
}

func TestRedirectHandler_NotFound(t *testing.T) {
	s := &mockStore{
		resolveFn: func(code string) (string, error) { return "", store.ErrNotFound },
	}
	r := newTestRouter(s)
	req := httptest.NewRequest(http.MethodGet, "/noexist", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestRedirectHandler_Expired(t *testing.T) {
	s := &mockStore{
		resolveFn: func(code string) (string, error) { return "", store.ErrExpired },
	}
	r := newTestRouter(s)
	req := httptest.NewRequest(http.MethodGet, "/expired", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusGone {
		t.Fatalf("expected 410, got %d", w.Code)
	}
	var resp errorResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error != "short link has expired" {
		t.Fatalf("unexpected error: %q", resp.Error)
	}
}

func TestStatsHandler_OK(t *testing.T) {
	s := &mockStore{
		statsFn: func(code string) (*store.StatsResponse, error) {
			return &store.StatsResponse{
				ShortCode:   code,
				OriginalURL: "https://example.com",
				TotalClicks: 5,
			}, nil
		},
	}
	r := newTestRouter(s)
	req := httptest.NewRequest(http.MethodGet, "/abc123/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp store.StatsResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.TotalClicks != 5 {
		t.Fatalf("expected 5 clicks, got %d", resp.TotalClicks)
	}
}

func TestStatsHandler_NotFound(t *testing.T) {
	s := &mockStore{
		statsFn: func(code string) (*store.StatsResponse, error) {
			return nil, errors.New("short code not found")
		},
	}
	r := newTestRouter(s)
	req := httptest.NewRequest(http.MethodGet, "/noexist/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestStatsHandler_ExpiresAt(t *testing.T) {
	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	s := &mockStore{
		statsFn: func(code string) (*store.StatsResponse, error) {
			return &store.StatsResponse{
				ShortCode:   code,
				OriginalURL: "https://example.com",
				TotalClicks: 1,
				ExpiresAt:   &expiresAt,
			}, nil
		},
	}
	r := newTestRouter(s)
	req := httptest.NewRequest(http.MethodGet, "/abc123/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	if _, ok := body["expires_at"]; !ok {
		t.Fatal("expected expires_at field in stats response")
	}
}

func TestQRHandler_Found(t *testing.T) {
	s := &mockStore{
		resolveFn: func(code string) (string, error) { return "https://example.com", nil },
	}
	r := newTestRouter(s)
	req := httptest.NewRequest(http.MethodGet, "/abc123/qr", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("expected Content-Type image/png, got %q", ct)
	}
	if w.Body.Len() == 0 {
		t.Fatal("expected non-empty PNG body")
	}
}

func TestQRHandler_NotFound(t *testing.T) {
	s := &mockStore{
		resolveFn: func(code string) (string, error) { return "", store.ErrNotFound },
	}
	r := newTestRouter(s)
	req := httptest.NewRequest(http.MethodGet, "/noexist/qr", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	var resp errorResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error != "short code not found" {
		t.Fatalf("unexpected error: %q", resp.Error)
	}
}

func TestQRHandler_Expired(t *testing.T) {
	s := &mockStore{
		resolveFn: func(code string) (string, error) { return "", store.ErrExpired },
	}
	r := newTestRouter(s)
	req := httptest.NewRequest(http.MethodGet, "/expired/qr", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusGone {
		t.Fatalf("expected 410, got %d", w.Code)
	}
	var resp errorResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error != "short link has expired" {
		t.Fatalf("unexpected error: %q", resp.Error)
	}
}
