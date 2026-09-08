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
		port = "8006"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from notification-gateway")
	})

	log.Printf("notification-gateway listening on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
