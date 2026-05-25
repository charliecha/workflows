package store

import (
	"sync"
	"testing"
	"time"
)

func TestShorten_NewURL(t *testing.T) {
	s := NewStore()
	code, _, err := s.Shorten("https://example.com", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(code) != codeLen {
		t.Fatalf("expected code length %d, got %d", codeLen, len(code))
	}
}

func TestShorten_Idempotent(t *testing.T) {
	s := NewStore()
	url := "https://example.com/idempotent"
	code1, _, _ := s.Shorten(url, 0)
	code2, _, _ := s.Shorten(url, 0)
	if code1 != code2 {
		t.Fatalf("expected same code, got %q and %q", code1, code2)
	}
}

func TestResolve_Found(t *testing.T) {
	s := NewStore()
	original := "https://example.com/resolve"
	code, _, _ := s.Shorten(original, 0)
	got, err := s.Resolve(code)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != original {
		t.Fatalf("expected %q, got %q", original, got)
	}
}

func TestResolve_NotFound(t *testing.T) {
	s := NewStore()
	_, err := s.Resolve("noexist")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRecordAccess_CircularBuffer(t *testing.T) {
	s := NewStore()
	url := "https://example.com/circ"
	code, _, _ := s.Shorten(url, 0)

	// write 101 records
	for i := 0; i < 101; i++ {
		s.RecordAccess(code, "ref"+string(rune('a'+i%26)), "ua")
	}

	stats, err := s.Stats(code)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stats.Accesses) != maxAccesses {
		t.Fatalf("expected %d accesses, got %d", maxAccesses, len(stats.Accesses))
	}
	if stats.TotalClicks != 101 {
		t.Fatalf("expected 101 clicks, got %d", stats.TotalClicks)
	}
}

func TestStats_ClickCount(t *testing.T) {
	s := NewStore()
	url := "https://example.com/concurrent"
	code, _, _ := s.Shorten(url, 0)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.RecordAccess(code, "", "ua")
		}()
	}
	wg.Wait()

	stats, _ := s.Stats(code)
	if stats.TotalClicks != 100 {
		t.Fatalf("expected 100 clicks, got %d", stats.TotalClicks)
	}
}

func TestStats_NotFound(t *testing.T) {
	s := NewStore()
	_, err := s.Stats("noexist")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestShorten_Conflict(t *testing.T) {
	s := NewStore()
	targetURL := "https://example.com/conflict"

	// Pre-seed attempt=0 code with a different URL to force collision
	code0 := generateCode(targetURL, 0)
	s.links[code0] = &shortLink{
		shortCode:   code0,
		originalURL: "https://other.com",
	}
	s.urls["https://other.com"] = code0

	// Shorten should fall through to attempt=1 and return a different code
	code, _, err := s.Shorten(targetURL, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code == code0 {
		t.Fatalf("expected a different code after collision, got same code %q", code)
	}
	if len(code) != codeLen {
		t.Fatalf("expected code length %d, got %d", codeLen, len(code))
	}
	got, err := s.Resolve(code)
	if err != nil {
		t.Fatalf("unexpected resolve error: %v", err)
	}
	if got != targetURL {
		t.Fatalf("expected %q, got %q", targetURL, got)
	}
}

func TestShorten_DefaultTTL(t *testing.T) {
	s := NewStore()
	before := time.Now().UTC()
	_, expiresAt, err := s.Shorten("https://example.com/default-ttl", 0)
	after := time.Now().UTC()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	low := before.Add(defaultTTL - time.Second)
	high := after.Add(defaultTTL + time.Second)
	if expiresAt.Before(low) || expiresAt.After(high) {
		t.Fatalf("expiresAt %v not in expected range [%v, %v]", expiresAt, low, high)
	}
}

func TestShorten_CustomTTL(t *testing.T) {
	s := NewStore()
	before := time.Now().UTC()
	_, expiresAt, err := s.Shorten("https://example.com/custom-ttl", 60)
	after := time.Now().UTC()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	low := before.Add(59 * time.Second)
	high := after.Add(61 * time.Second)
	if expiresAt.Before(low) || expiresAt.After(high) {
		t.Fatalf("expiresAt %v not in expected range [%v, %v]", expiresAt, low, high)
	}
}

func TestResolve_Expired(t *testing.T) {
	s := NewStore()
	code, _, err := s.Shorten("https://example.com/expire-soon", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	time.Sleep(1100 * time.Millisecond)
	_, err = s.Resolve(code)
	if err != ErrExpired {
		t.Fatalf("expected ErrExpired, got %v", err)
	}
}

func TestResolve_ZeroExpiresAt(t *testing.T) {
	s := NewStore()
	// Directly insert a link with zero expiresAt (legacy link)
	s.links["legacy"] = &shortLink{
		shortCode:   "legacy",
		originalURL: "https://example.com/legacy",
		createdAt:   time.Now().UTC(),
	}
	s.urls["https://example.com/legacy"] = "legacy"

	got, err := s.Resolve("legacy")
	if err != nil {
		t.Fatalf("expected no error for zero expiresAt, got %v", err)
	}
	if got != "https://example.com/legacy" {
		t.Fatalf("unexpected url: %q", got)
	}
}

func TestShorten_AfterExpiry_ReturnsFreshCode(t *testing.T) {
	s := NewStore()
	url := "https://example.com/reuse"
	_, _, err := s.Shorten(url, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	time.Sleep(1100 * time.Millisecond)

	// After expiry, Shorten should create a fresh entry and return a usable code.
	code2, expiresAt2, err := s.Shorten(url, 60)
	if err != nil {
		t.Fatalf("unexpected error on re-shorten after expiry: %v", err)
	}
	// The new code must be resolvable.
	got, err := s.Resolve(code2)
	if err != nil {
		t.Fatalf("expected resolve to succeed after re-shorten, got %v", err)
	}
	if got != url {
		t.Fatalf("unexpected url: %q", got)
	}
	// New expiresAt should be ~60s from now, not in the past.
	if !time.Now().UTC().Before(expiresAt2) {
		t.Fatalf("new expiresAt %v should be in the future", expiresAt2)
	}
}

func TestShorten_LiveLink_TTLNotUpdated(t *testing.T) {
	s := NewStore()
	url := "https://example.com/ttl-once"
	_, expiresAt1, _ := s.Shorten(url, 3600)

	// Calling Shorten again with a different TTL on a live link returns the original expiresAt.
	_, expiresAt2, _ := s.Shorten(url, 60)
	if !expiresAt1.Equal(expiresAt2) {
		t.Fatalf("expected TTL to be set-once for live links, got %v vs %v", expiresAt1, expiresAt2)
	}
}
