// Command server runs the opusflow backend.
package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/yaremam/opusflow/backend/internal/auth"
	"github.com/yaremam/opusflow/backend/internal/db"
	"github.com/yaremam/opusflow/backend/internal/httpserver"
	"github.com/yaremam/opusflow/backend/internal/library"
	"github.com/yaremam/opusflow/backend/internal/library/enrich"
	"github.com/yaremam/opusflow/backend/internal/library/organize"
)

// migrateRetryDelay paces retries of the startup migration against a
// Postgres that isn't reachable yet (e.g. still starting up alongside this
// process in Docker Compose).
const migrateRetryDelay = 2 * time.Second

// enrichUserAgent identifies this application to MusicBrainz, Cover Art
// Archive, and Wikidata/Wikipedia per their usage policies — all three
// expect a descriptive User-Agent naming the app and a contact.
const enrichUserAgent = "opusflow/0.1 (+https://github.com/yaremam/opusflow)"

// defaultDataDir is where every shipped compose file mounts the music
// volume — see DATA_DIR below.
const defaultDataDir = "/data"

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	staticDir := os.Getenv("STATIC_DIR")
	artworkDir := os.Getenv("ARTWORK_DIR")
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = defaultDataDir
	}
	version := os.Getenv("VERSION")
	buildDate := os.Getenv("BUILD_DATE")

	conn, err := db.Connect(databaseURL())
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	defer conn.Close()

	store := library.NewStore(conn)
	svc := library.NewService(store, organize.CopyJob{})
	authStore := auth.NewStore(conn)

	// DATA_DIR defaults to /data — every compose file (both root and
	// deploy/) mounts the music volume there, so an operator should never
	// have to set this themselves. Still overridable for a plain `go run`
	// outside the Docker image, where /data won't exist on the host at all.
	svc.SetBrowseRoot(dataDir)

	// The interactive "look up metadata" flow (TDR 012) needs only the
	// rate-limited MusicBrainz text-search client, none of the image
	// storage ARTWORK_DIR gates below — so it's wired up unconditionally,
	// unlike the background enrichment job.
	svc.SetMusicBrainzSearch(enrich.NewMusicBrainz(enrichUserAgent))

	// ARTWORK_DIR is optional the same way STATIC_DIR is: unset means
	// "this feature is off" (embedded-art extraction skipped, no
	// enrichment job) rather than an error, e.g. for a plain `go run`
	// outside the Docker image where no artwork volume is mounted.
	var job *enrich.Job
	if artworkDir != "" {
		images := enrich.NewImageStore(artworkDir)
		store.SetImages(images)
		svc.SetImages(images)
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
	// succeed, for the same reason: the tables it needs don't exist until
	// then.
	go func() {
		migrateWithRetry(conn)
		if job != nil {
			sum := job.Run(context.Background())
			log.Printf("enrich: startup run: %d found, %d not found, %d failed", sum.Found, sum.NotFound, sum.Failed)
		}
	}()

	log.Printf("listening on :%s", port)
	if err := http.ListenAndServe(":"+port, httpserver.New(staticDir, artworkDir, version, buildDate, svc, authStore)); err != nil {
		log.Fatal(err)
	}
}

// databaseURL returns DATABASE_URL as-is if it's set — an explicit
// connection string, for connecting to a Postgres this compose file doesn't
// manage. Otherwise it builds one from POSTGRES_USER/POSTGRES_DB
// (defaulting POSTGRES_HOST to "postgres", the compose service name, and
// POSTGRES_PORT to 5432). POSTGRES_PASSWORD is optional: unset entirely in
// deploy/docker-compose.yml, since postgres isn't reachable from outside
// the compose network and is configured there for trust authentication
// (POSTGRES_HOST_AUTH_METHOD=trust) rather than a password — set it only
// when pointing at a Postgres that actually requires one. Built via
// net/url.URL rather than string formatting so a user/password containing
// URL-special characters round-trips correctly.
func databaseURL() string {
	if v := os.Getenv("DATABASE_URL"); v != "" {
		return v
	}

	host := os.Getenv("POSTGRES_HOST")
	if host == "" {
		host = "postgres"
	}
	port := os.Getenv("POSTGRES_PORT")
	if port == "" {
		port = "5432"
	}

	user := os.Getenv("POSTGRES_USER")
	userinfo := url.User(user)
	if password := os.Getenv("POSTGRES_PASSWORD"); password != "" {
		userinfo = url.UserPassword(user, password)
	}

	u := url.URL{
		Scheme: "postgres",
		User:   userinfo,
		Host:   host + ":" + port,
		Path:   "/" + os.Getenv("POSTGRES_DB"),
	}
	q := u.Query()
	q.Set("sslmode", "disable")
	u.RawQuery = q.Encode()
	return u.String()
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
