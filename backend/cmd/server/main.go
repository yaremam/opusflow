// Command server runs the opusflow backend.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/yaremam/opusflow/backend/internal/httpserver"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	staticDir := os.Getenv("STATIC_DIR")

	log.Printf("listening on :%s", port)
	if err := http.ListenAndServe(":"+port, httpserver.New(staticDir)); err != nil {
		log.Fatal(err)
	}
}
