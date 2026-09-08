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
		port = "8007"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from traffic-control")
	})

	log.Printf("traffic-control listening on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
