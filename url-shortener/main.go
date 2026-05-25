package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/user/url-shortener/handler"
	"github.com/user/url-shortener/store"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	baseURL := fmt.Sprintf("http://localhost:%s", port)

	s := store.NewStore()
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	r.Post("/shorten", handler.ShortenHandler(s, baseURL))
	r.Get("/{code}", handler.RedirectHandler(s))
	r.Get("/{code}/stats", handler.StatsHandler(s))

	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
