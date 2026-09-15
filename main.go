package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"

	"net/url"
	"github.com/jackc/pgx/v5/pgxpool"
)

type URL struct{
	URL string `json:"url"`
}

type Code struct {
	Code string `json:"code"`
}

func main() {

	ctx := context.Background()

	pool, err := connectDB(ctx, "postgres://myuser:mypassword@localhost:5432/url-shortener")
	if err != nil {
		log.Fatalf("db connection error: %v", err)
	}

	defer pool.Close()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /{code}", func(w http.ResponseWriter, r *http.Request) {

		code := r.PathValue("code")

		fullURL, err := getURL(code, r.Context(), pool)
		if err != nil {
			http.Error(w, "Error: Short Code not found", http.StatusNotFound)
			return
		}

		http.Redirect(w, r, fullURL, http.StatusFound)
	})

	mux.HandleFunc("POST /shorten", func(w http.ResponseWriter, r *http.Request) {

		var u URL

		err := json.NewDecoder(r.Body).Decode(&u)
		if err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		parsed, err := url.ParseRequestURI(u.URL)
		if err != nil || parsed.Host == "" {
			http.Error(w, "Invalid URL: ", http.StatusBadRequest)
			return
		}

		randomCode, err := shorten(u.URL, r.Context(), pool)
		if err != nil {
			fmt.Fprint(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string {
			"code": randomCode,
		})
	})

	http.ListenAndServe(":8080", mux)
}

func generateCode() string {

	allCharacters := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	sixRandomChar := ""

	for i := 0; i < 6; i++ {
		sixRandomChar += string(allCharacters[rand.IntN(len(allCharacters))])
	}

	return sixRandomChar
}

func connectDB(ctx context.Context, connString string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("error creating connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	return pool, nil
}

func createTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS urls (
			shortCode TEXT PRIMARY KEY,
			fullURL TEXT NOT NULL,
			created_at TIMESTAMPTZ DEFAULT now()
		)`)

	if err != nil {
		return fmt.Errorf("error creating table: %w", err)
	}

	return nil
}

func insertNewURL(randomCode string, fullURL string, ctx context.Context, pool *pgxpool.Pool) error {

	_, err := pool.Exec(ctx, "INSERT INTO urls (shortCode, fullURL) VALUES ($1, $2)", randomCode, fullURL)
	if err != nil {
		return fmt.Errorf("Insert Failed: %w", err)
	}

	return nil
}

func shorten(fullURL string, ctx context.Context, pool *pgxpool.Pool) (string, error) {

	err := createTable(ctx, pool)
	if err != nil {
		log.Fatal(err)
	}

	randomCode := generateCode()
	err = insertNewURL(randomCode, fullURL, ctx, pool)
	if err != nil {
		return "",fmt.Errorf("Error inserting: %w", err)
	}

	return randomCode, nil
}

func getURL(code string, ctx context.Context, pool *pgxpool.Pool) (string, error) {
	var fullURL string
	err := pool.QueryRow(ctx, "SELECT fullURL FROM urls WHERE shortCode = $1", code).Scan(&fullURL)
	if err != nil {
		return "", fmt.Errorf("Select failed: %v", err)
	}
	return fullURL, nil
}
