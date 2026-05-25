package store

import (
	"sync"
	"testing"
)

func TestShorten_NewURL(t *testing.T) {
	s := NewStore()
	code, err := s.Shorten("https://example.com")
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
	code1, _ := s.Shorten(url)
	code2, _ := s.Shorten(url)
	if code1 != code2 {
		t.Fatalf("expected same code, got %q and %q", code1, code2)
	}
}

func TestResolve_Found(t *testing.T) {
	s := NewStore()
	original := "https://example.com/resolve"
	code, _ := s.Shorten(original)
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
	code, _ := s.Shorten(url)

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
	code, _ := s.Shorten(url)

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
	code, err := s.Shorten(targetURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code == code0 {
		t.Fatalf("expected a different code after collision, got same code %q", code)
	}
	if len(code) != codeLen {
		t.Fatalf("expected code length %d, got %d", codeLen, len(code))
	}
	// Verify it resolves correctly
	got, err := s.Resolve(code)
	if err != nil {
		t.Fatalf("unexpected resolve error: %v", err)
	}
	if got != targetURL {
		t.Fatalf("expected %q, got %q", targetURL, got)
	}
}
