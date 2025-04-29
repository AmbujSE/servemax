package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

// Struct to hold stateful data
type apiConfig struct {
	fileserverHits atomic.Int32
	db             *sql.DB
	platform       string
}

// Handler to validate a chirp
func (cfg *apiConfig) validateChirpHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Parse the JSON body
	var requestBody struct {
		Chirp string `json:"chirp"`
	}
	err := json.NewDecoder(r.Body).Decode(&requestBody)
	if err != nil || requestBody.Chirp == "" {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON or missing chirp")
		return
	}

	// Validate the chirp length
	if len(requestBody.Chirp) > 140 {
		respondWithError(w, http.StatusBadRequest, "Chirp exceeds 140 characters")
		return
	}

	// Respond with success
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "Chirp is valid"})
}

// Middleware to increment file server hits
func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

// Handler to return admin metrics
func (cfg *apiConfig) adminMetricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	metrics := map[string]interface{}{
		"file_server_hits": cfg.fileserverHits.Load(),
	}

	respondWithJSON(w, http.StatusOK, metrics)
}

// Helper function to respond with an error
func respondWithError(w http.ResponseWriter, code int, message string) {
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// Helper function to respond with JSON
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(payload)
}

// Struct for User
type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

// Handler to create a user
func (cfg *apiConfig) createUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Parse the JSON body
	var requestBody struct {
		Email string `json:"email"`
	}
	err := json.NewDecoder(r.Body).Decode(&requestBody)
	if err != nil || requestBody.Email == "" {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON or missing email")
		return
	}

	// Insert the user into the database
	var user User
	query := `
        INSERT INTO users (id, created_at, updated_at, email)
        VALUES ($1, $2, $3, $4)
        RETURNING id, created_at, updated_at, email`
	err = cfg.db.QueryRowContext(r.Context(), query,
		uuid.New(), time.Now(), time.Now(), requestBody.Email,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt, &user.Email)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	// Respond with the created user
	respondWithJSON(w, http.StatusCreated, user)
}

// Handler to reset all users
func (cfg *apiConfig) resetUsersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Check if the platform is "dev"
	if cfg.platform != "dev" {
		respondWithError(w, http.StatusForbidden, "Forbidden")
		return
	}

	// Delete all users from the database
	_, err := cfg.db.ExecContext(r.Context(), "DELETE FROM users")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to reset users")
		return
	}

	// Respond with success
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "All users deleted"})
}

func main() {
	// Load environment variables from .env file
	godotenv.Load()

	// Read the PLATFORM environment variable
	platform := os.Getenv("PLATFORM")

	// Connect to the database
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		panic(err)
	}

	// Create a new ServeMux
	mux := http.NewServeMux()

	// Create an instance of apiConfig
	apiCfg := &apiConfig{
		db:       db,
		platform: platform,
	}

	// Add the validate chirp endpoint (POST only) under /api
	mux.HandleFunc("/api/validate_chirp", apiCfg.validateChirpHandler)

	// Add the readiness endpoint (GET only) under /api
	mux.HandleFunc("/api/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		respondWithJSON(w, http.StatusOK, map[string]string{"status": "OK"})
	})

	// Use http.FileServer to serve files from the /app/ path
	fileServer := http.FileServer(http.Dir("."))
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", fileServer)))

	// Add the admin metrics endpoint (GET only) under /admin
	mux.HandleFunc("/admin/metrics", apiCfg.adminMetricsHandler)

	// Add the admin reset endpoint (POST only) under /admin
	mux.HandleFunc("/admin/reset", apiCfg.resetUsersHandler)

	// Add the create user endpoint (POST only) under /api
	mux.HandleFunc("/api/users", apiCfg.createUserHandler)

	// Create a new HTTP server
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Start the server
	err = server.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
