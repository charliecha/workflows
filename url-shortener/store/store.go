package store

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"math/big"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

const base62Chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const codeLen = 6
const maxAccesses = 100
const defaultTTL = 30 * 24 * time.Hour

var ErrNotFound = errors.New("short code not found")
var ErrExpired = errors.New("short link has expired")

type AccessRecord struct {
	Time      time.Time `json:"time"`
	Referer   string    `json:"referer"`
	UserAgent string    `json:"user_agent"`
}

type StatsResponse struct {
	ShortCode   string         `json:"short_code"`
	OriginalURL string         `json:"original_url"`
	TotalClicks int64          `json:"total_clicks"`
	CreatedAt   time.Time      `json:"created_at"`
	ExpiresAt   *time.Time     `json:"expires_at,omitempty"`
	Accesses    []AccessRecord `json:"accesses"`
}

type shortLink struct {
	shortCode   string
	originalURL string
	createdAt   time.Time
	expiresAt   time.Time
	clicks      atomic.Int64
	accesses    [maxAccesses]AccessRecord
	accessHead  int
	accessCount int
	mu          sync.Mutex
}

func (l *shortLink) recordAccess(referer, userAgent string) {
	l.clicks.Add(1)
	l.mu.Lock()
	defer l.mu.Unlock()
	l.accesses[l.accessHead] = AccessRecord{
		Time:      time.Now().UTC(),
		Referer:   referer,
		UserAgent: userAgent,
	}
	l.accessHead = (l.accessHead + 1) % maxAccesses
	if l.accessCount < maxAccesses {
		l.accessCount++
	}
}

func (l *shortLink) getAccesses() []AccessRecord {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.accessCount == 0 {
		return nil
	}
	result := make([]AccessRecord, l.accessCount)
	start := (l.accessHead - l.accessCount + maxAccesses) % maxAccesses
	for i := 0; i < l.accessCount; i++ {
		result[i] = l.accesses[(start+i)%maxAccesses]
	}
	return result
}

type Storer interface {
	Shorten(originalURL string, ttl int) (string, time.Time, error)
	Resolve(shortCode string) (string, error)
	RecordAccess(shortCode, referer, userAgent string) error
	Stats(shortCode string) (*StatsResponse, error)
}

type Store struct {
	mu    sync.RWMutex
	links map[string]*shortLink
	urls  map[string]string
}

func NewStore() *Store {
	return &Store{
		links: make(map[string]*shortLink),
		urls:  make(map[string]string),
	}
}

func (s *Store) Shorten(originalURL string, ttl int) (string, time.Time, error) {
	if ttl == 0 {
		ttl = int(defaultTTL.Seconds())
	}
	expiresAt := time.Now().UTC().Add(time.Duration(ttl) * time.Second)

	s.mu.RLock()
	if code, ok := s.urls[originalURL]; ok {
		existing := s.links[code]
		// If the existing link is still live, return it as-is (TTL is set-once per URL).
		// If expired, fall through to create a fresh entry below.
		if existing.expiresAt.IsZero() || time.Now().UTC().Before(existing.expiresAt) {
			s.mu.RUnlock()
			return code, existing.expiresAt, nil
		}
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	// re-check after acquiring write lock
	if code, ok := s.urls[originalURL]; ok {
		existing := s.links[code]
		if existing.expiresAt.IsZero() || time.Now().UTC().Before(existing.expiresAt) {
			return code, existing.expiresAt, nil
		}
		// Expired: remove stale entry so the loop can mint a new code.
		delete(s.links, code)
		delete(s.urls, originalURL)
	}
	for attempt := 0; attempt < 10; attempt++ {
		code := generateCode(originalURL, attempt)
		existing, ok := s.links[code]
		if !ok {
			s.links[code] = &shortLink{
				shortCode:   code,
				originalURL: originalURL,
				createdAt:   time.Now().UTC(),
				expiresAt:   expiresAt,
			}
			s.urls[originalURL] = code
			return code, expiresAt, nil
		}
		if existing.originalURL == originalURL {
			return code, existing.expiresAt, nil
		}
	}
	return "", time.Time{}, errors.New("failed to generate unique short code")
}

func (s *Store) Resolve(shortCode string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	link, ok := s.links[shortCode]
	if !ok {
		return "", ErrNotFound
	}
	if !link.expiresAt.IsZero() && time.Now().UTC().After(link.expiresAt) {
		return "", ErrExpired
	}
	return link.originalURL, nil
}

func (s *Store) RecordAccess(shortCode, referer, userAgent string) error {
	s.mu.RLock()
	link, ok := s.links[shortCode]
	s.mu.RUnlock()
	if !ok {
		return ErrNotFound
	}
	link.recordAccess(referer, userAgent)
	return nil
}

func (s *Store) Stats(shortCode string) (*StatsResponse, error) {
	s.mu.RLock()
	link, ok := s.links[shortCode]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrNotFound
	}
	resp := &StatsResponse{
		ShortCode:   link.shortCode,
		OriginalURL: link.originalURL,
		TotalClicks: link.clicks.Load(),
		CreatedAt:   link.createdAt,
		Accesses:    link.getAccesses(),
	}
	if !link.expiresAt.IsZero() {
		t := link.expiresAt
		resp.ExpiresAt = &t
	}
	return resp, nil
}

func generateCode(originalURL string, attempt int) string {
	input := originalURL + ":" + strconv.Itoa(attempt)
	hash := md5.Sum([]byte(input))
	hexStr := hex.EncodeToString(hash[:])
	code := toBase62(hexStr[:8])
	if len(code) >= codeLen {
		return code[:codeLen]
	}
	// pad with zeros if too short (extremely rare)
	for len(code) < codeLen {
		code = "0" + code
	}
	return code
}

func toBase62(hexStr string) string {
	n := new(big.Int)
	n.SetString(hexStr, 16)
	if n.Sign() == 0 {
		return "0"
	}
	base := big.NewInt(62)
	mod := new(big.Int)
	var result []byte
	for n.Sign() > 0 {
		n.DivMod(n, base, mod)
		result = append(result, base62Chars[mod.Int64()])
	}
	// reverse
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return string(result)
}
