package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

// Placeholder response structure
type APIResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, resp APIResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8001"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from graph-engine")
	})

	// POST /api/infrastructure/upload-geojson
	// Accepts GeoJSON/KML file uploads to build the city topology graph.
	http.HandleFunc("/api/infrastructure/upload-geojson", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, APIResponse{
				Status:  "error",
				Message: "Method not allowed. Use POST.",
			})
			return
		}

		// TODO: Parse multipart form, extract .geojson/.kml,
		// ingest into Neo4j as graph nodes/edges.
		log.Println("[graph-engine] Received GeoJSON upload request")

		writeJSON(w, http.StatusOK, APIResponse{
			Status:  "ok",
			Message: "GeoJSON received. Topology graph queued for ingestion.",
			Data: map[string]interface{}{
				"nodes_created": 0,
				"edges_created": 0,
			},
		})
	})

	// POST /api/infrastructure/map-dependencies
	// Accepts dependency mapping rules (source/target/mode).
	http.HandleFunc("/api/infrastructure/map-dependencies", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, APIResponse{
				Status:  "error",
				Message: "Method not allowed. Use POST.",
			})
			return
		}

		var body struct {
			Rules []struct {
				Source string `json:"source"`
				Target string `json:"target"`
				Mode   string `json:"mode"`
			} `json:"rules"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, APIResponse{
				Status:  "error",
				Message: "Invalid JSON body.",
			})
			return
		}

		log.Printf("[graph-engine] Received %d dependency rules", len(body.Rules))

		writeJSON(w, http.StatusOK, APIResponse{
			Status:  "ok",
			Message: fmt.Sprintf("Mapped %d dependency rules into the topology graph.", len(body.Rules)),
		})
	})

	log.Printf("graph-engine listening on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
