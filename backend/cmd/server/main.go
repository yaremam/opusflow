// Command server runs the opusflow backend.
package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/yaremam/opusflow/backend/internal/db"
	"github.com/yaremam/opusflow/backend/internal/httpserver"
	"github.com/yaremam/opusflow/backend/internal/library"
	"github.com/yaremam/opusflow/backend/internal/library/scan"
)

// migrateRetryDelay paces retries of the startup migration against a
// Postgres that isn't reachable yet (e.g. still starting up alongside this
// process in Docker Compose).
const migrateRetryDelay = 2 * time.Second

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	staticDir := os.Getenv("STATIC_DIR")
	roots := library.ParseRoots(os.Getenv("LIBRARY_ROOTS"))

	conn, err := db.Connect(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	defer conn.Close()

	store := library.NewStore(conn)
	scanner := scan.NewScanner(store)
	svc := library.NewService(roots, store, scanner)

	// Migrations run in the background rather than blocking startup: /health
	// (and the process as a whole) should come up regardless of how long
	// Postgres takes to become reachable. Until migrations land, library
	// endpoints will simply return errors on the queries that need them.
	go migrateWithRetry(conn)

	log.Printf("listening on :%s", port)
	if err := http.ListenAndServe(":"+port, httpserver.New(staticDir, svc)); err != nil {
		log.Fatal(err)
	}
}

func migrateWithRetry(conn *sql.DB) {
	for {
		if err := db.Migrate(conn); err != nil {
			log.Printf("running migrations (will retry in %s): %v", migrateRetryDelay, err)
			time.Sleep(migrateRetryDelay)
			continue
		}
		log.Print("migrations applied")
		return
	}
}
