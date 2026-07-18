package main

import (
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var pageTemplate = template.Must(template.New("home").Parse(`
<!DOCTYPE html>
<html lang="es">
<head>
	<meta charset="UTF-8">
	<title>Plataforma institucional</title>
</head>
<body>
	<h1>Plataforma institucional en Go</h1>

	<p>La aplicación está conectada con PostgreSQL.</p>

	<p>
		Instancia que respondió:
		<strong>{{.Hostname}}</strong>
	</p>

	<p>
		Visitas registradas en PostgreSQL:
		<strong>{{.Visits}}</strong>
	</p>
</body>
</html>
`))

type pageData struct {
	Hostname string
	Visits   int64
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}

func readSecret(valueVariable, fileVariable string) (string, error) {
	if value := os.Getenv(valueVariable); value != "" {
		return value, nil
	}

	filePath := os.Getenv(fileVariable)
	if filePath == "" {
		return "", fmt.Errorf(
			"no se definió %s ni %s",
			valueVariable,
			fileVariable,
		)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("leer secreto %s: %w", filePath, err)
	}

	return strings.TrimSpace(string(content)), nil
}

func connectDatabase() *sql.DB {
	password, err := readSecret("DB_PASSWORD", "DB_PASSWORD_FILE")
	if err != nil {
		log.Fatal(err)
	}

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		env("DB_HOST", "postgres"),
		env("DB_PORT", "5432"),
		env("DB_USER", "plataforma"),
		password,
		env("DB_NAME", "plataforma"),
		env("DB_SSLMODE", "disable"),
	)

	const maximumAttempts = 30

	for attempt := 1; attempt <= maximumAttempts; attempt++ {
		db, openErr := sql.Open("pgx", dsn)
		if openErr == nil {
			ctx, cancel := context.WithTimeout(
				context.Background(),
				3*time.Second,
			)

			pingErr := db.PingContext(ctx)
			cancel()

			if pingErr == nil {
				log.Println("Conexión con PostgreSQL establecida")
				return db
			}

			_ = db.Close()
			openErr = pingErr
		}

		log.Printf(
			"PostgreSQL todavía no está disponible, intento %d/%d: %v",
			attempt,
			maximumAttempts,
			openErr,
		)

		time.Sleep(2 * time.Second)
	}

	log.Fatal("no fue posible conectarse con PostgreSQL")
	return nil
}

func initializeDatabase(db *sql.DB) {
	query := `
		CREATE TABLE IF NOT EXISTS counters (
			name  TEXT PRIMARY KEY,
			value BIGINT NOT NULL DEFAULT 0
		);

		INSERT INTO counters (name, value)
		VALUES ('visits', 0)
		ON CONFLICT (name) DO NOTHING;
	`

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	if _, err := db.ExecContext(ctx, query); err != nil {
		log.Fatalf("inicializar base de datos: %v", err)
	}
}

func homeHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		ctx, cancel := context.WithTimeout(
			r.Context(),
			3*time.Second,
		)
		defer cancel()

		var visits int64

		err := db.QueryRowContext(
			ctx,
			`
				UPDATE counters
				SET value = value + 1
				WHERE name = 'visits'
				RETURNING value
			`,
		).Scan(&visits)

		if err != nil {
			http.Error(
				w,
				"Error consultando PostgreSQL",
				http.StatusInternalServerError,
			)
			log.Printf("actualizar contador: %v", err)
			return
		}

		hostname, err := os.Hostname()
		if err != nil {
			hostname = "desconocido"
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Backend-Instance", hostname)

		err = pageTemplate.Execute(w, pageData{
			Hostname: hostname,
			Visits:   visits,
		})

		if err != nil {
			log.Printf("generar HTML: %v", err)
		}
	}
}

func liveHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("alive\n"))
}

func readyHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(
			r.Context(),
			2*time.Second,
		)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			http.Error(
				w,
				"PostgreSQL no disponible",
				http.StatusServiceUnavailable,
			)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready\n"))
	}
}

func main() {
	db := connectDatabase()
	defer db.Close()

	initializeDatabase(db)

	mux := http.NewServeMux()
	mux.HandleFunc("/", homeHandler(db))
	mux.HandleFunc("/health/live", liveHandler)
	mux.HandleFunc("/health/ready", readyHandler(db))

	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Println("Servidor Go escuchando en el puerto 8080")

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
