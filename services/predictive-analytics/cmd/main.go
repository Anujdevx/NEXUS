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
		port = "8009"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from predictive-analytics")
	})

	log.Printf("predictive-analytics listening on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
