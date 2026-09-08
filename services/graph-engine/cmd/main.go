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
		port = "8001"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from graph-engine")
	})

	log.Printf("graph-engine listening on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
