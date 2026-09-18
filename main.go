package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/joho/godotenv"
)

type Url struct {
	Url string `json:"url"`
}

type ShortCode struct {
	Code string `json:"code"`
}

func main() {
	ctx := context.Background()
	godotenv.Load()

	// Connects to the database
	pool, err := connectDB(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("db connection error: %v", err)
	}

	defer pool.Close()

	// Create the table urls if it does not exist
	if err := createTable(ctx, pool); err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /{code}", func(w http.ResponseWriter, r *http.Request) {

		code := r.PathValue("code")

		fullUrl, err := getUrl(r.Context(), pool, code)
		if err != nil {
			http.Error(w, "Error: Short Code not found", http.StatusNotFound)
			return
		}

		http.Redirect(w, r, fullUrl, http.StatusFound)
	})

	mux.HandleFunc("POST /shorten", func(w http.ResponseWriter, r *http.Request) {

		var u Url

		err := json.NewDecoder(r.Body).Decode(&u)
		if err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		parsed, err := url.ParseRequestURI(u.Url)
		if err != nil || parsed.Host == "" {
			http.Error(w, "Invalid URL: ", http.StatusBadRequest)
			return
		}

		shortCode, err := shorten(r.Context(), u.Url, pool)
		if err != nil {
			log.Printf("shorten error: %v", err)
			http.Error(w, "failed to create short url", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(ShortCode{Code: shortCode})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Fatal(http.ListenAndServe(":"+port,corsMiddleware(mux)))
}

func generateShortCode() string {

	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	shortCode := make([]byte, 6)

	for i := 0; i < 6; i++ {
		shortCode[i] = chars[rand.IntN(len(chars))]
	}

	return string(shortCode)
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

func insertNewURL(ctx context.Context, randomCode string, fullURL string, pool *pgxpool.Pool) error {

	_, err := pool.Exec(ctx, "INSERT INTO urls (shortCode, fullURL) VALUES ($1, $2)", randomCode, fullURL)
	if err != nil {
		return fmt.Errorf("Insert Failed: %w", err)
	}

	return nil
}

func shorten(ctx context.Context, fullUrl string, pool *pgxpool.Pool) (string, error) {
	maxAttempt := 10
	for i := 0; i < maxAttempt; i++ {
		shortCode := generateShortCode()
		err := insertNewURL(ctx, shortCode, fullUrl, pool)
		if err == nil {
			return shortCode, nil
		}
		if !isUniqueViolation(err) {
			return "", fmt.Errorf("insert failed: %w", err)
		}
	}

	return "", fmt.Errorf("failed to generate a unique code after %d attempts", maxAttempt)
}

func getUrl(ctx context.Context, pool *pgxpool.Pool, code string) (string, error) {
	var fullURL string
	err := pool.QueryRow(ctx, "SELECT fullURL FROM urls WHERE shortCode = $1", code).Scan(&fullURL)
	if err != nil {
		return "", fmt.Errorf("Select failed: %w", err)
	}
	return fullURL, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" // Postgres unique_violation code
	}
	return false
}

var allowedOrigins = map[string]bool {
	"https://puggypotato.com": true,
	"https://tqyx.me": true,
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}