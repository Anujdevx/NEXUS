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
		port = "8008"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from environmental-monitor")
	})

	log.Printf("environmental-monitor listening on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
