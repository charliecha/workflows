package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
	"github.com/user/url-shortener/store"
)

const maxURLLength = 2048

type shortenRequest struct {
	URL string `json:"url"`
}

type shortenResponse struct {
	ShortCode string `json:"short_code"`
	ShortURL  string `json:"short_url"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

func validateURL(raw string) error {
	if raw == "" {
		return errors.New("url is required")
	}
	if len(raw) > maxURLLength {
		return fmt.Errorf("url exceeds maximum length of %d bytes", maxURLLength)
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("invalid URL: must start with http:// or https://")
	}
	return nil
}

func ShortenHandler(s store.Storer, baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req shortenRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if err := validateURL(req.URL); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		code, err := s.Shorten(req.URL)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate short code")
			return
		}
		writeJSON(w, http.StatusOK, shortenResponse{
			ShortCode: code,
			ShortURL:  baseURL + "/" + code,
		})
	}
}

func RedirectHandler(s store.Storer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := chi.URLParam(r, "code")
		originalURL, err := s.Resolve(code)
		if err != nil {
			writeError(w, http.StatusNotFound, "short code not found")
			return
		}
		s.RecordAccess(code, r.Header.Get("Referer"), r.Header.Get("User-Agent"))
		http.Redirect(w, r, originalURL, http.StatusFound)
	}
}

func StatsHandler(s store.Storer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := chi.URLParam(r, "code")
		stats, err := s.Stats(code)
		if err != nil {
			writeError(w, http.StatusNotFound, "short code not found")
			return
		}
		writeJSON(w, http.StatusOK, stats)
	}
}
