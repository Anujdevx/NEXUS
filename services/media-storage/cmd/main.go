package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8010"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from media-storage")
	})

	log.Printf("media-storage listening on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
