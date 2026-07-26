// Command server runs the opusflow backend.
package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/yaremam/opusflow/backend/internal/db"
	"github.com/yaremam/opusflow/backend/internal/httpserver"
	"github.com/yaremam/opusflow/backend/internal/library"
	"github.com/yaremam/opusflow/backend/internal/library/enrich"
	"github.com/yaremam/opusflow/backend/internal/library/scan"
)

// migrateRetryDelay paces retries of the startup migration against a
// Postgres that isn't reachable yet (e.g. still starting up alongside this
// process in Docker Compose).
const migrateRetryDelay = 2 * time.Second

// enrichUserAgent identifies this application to MusicBrainz, Cover Art
// Archive, and Wikidata/Wikipedia per their usage policies — all three
// expect a descriptive User-Agent naming the app and a contact.
const enrichUserAgent = "opusflow/0.1 (+https://github.com/yaremam/opusflow)"

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	staticDir := os.Getenv("STATIC_DIR")
	artworkDir := os.Getenv("ARTWORK_DIR")
	revision := os.Getenv("GIT_SHA")
	roots := library.ParseRoots(os.Getenv("LIBRARY_ROOTS"))

	conn, err := db.Connect(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	defer conn.Close()

	store := library.NewStore(conn)
	scanner := scan.NewScanner(store)
	svc := library.NewService(roots, store, scanner)

	// ARTWORK_DIR is optional the same way STATIC_DIR is: unset means
	// "this feature is off" (embedded-art extraction skipped, no
	// enrichment job) rather than an error, e.g. for a plain `go run`
	// outside the Docker image where no artwork volume is mounted.
	var job *enrich.Job
	if artworkDir != "" {
		images := enrich.NewImageStore(artworkDir)
		store.SetImages(images)
		job = enrich.NewJob(store,
			enrich.NewMusicBrainz(enrichUserAgent),
			enrich.NewCoverArtArchive(enrichUserAgent),
			enrich.NewWikidata(enrichUserAgent),
			images,
		)
		svc.SetEnricher(job)
	}

	// Migrations run in the background rather than blocking startup: /health
	// (and the process as a whole) should come up regardless of how long
	// Postgres takes to become reachable. Until migrations land, library
	// endpoints will simply return errors on the queries that need them.
	// The enrichment job's first run (TDR 003's startup trigger, covering a
	// library that predates this feature) is chained after migrations
	// succeed, for the same reason a scan-triggered run would fail before
	// then: the enrichment columns migration 0003 added don't exist yet.
	go func() {
		migrateWithRetry(conn)
		if job != nil {
			sum := job.Run(context.Background())
			log.Printf("enrich: startup run: %d found, %d not found, %d failed", sum.Found, sum.NotFound, sum.Failed)
		}
	}()

	log.Printf("listening on :%s", port)
	if err := http.ListenAndServe(":"+port, httpserver.New(staticDir, artworkDir, revision, svc)); err != nil {
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
