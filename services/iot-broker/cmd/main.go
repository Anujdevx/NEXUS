package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
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
		port = "8002"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from iot-broker")
	})

	// POST /api/iot/generate-webhook
	// Generates a unique webhook URL + bearer token for a given sensor type.
	http.HandleFunc("/api/iot/generate-webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, APIResponse{
				Status:  "error",
				Message: "Method not allowed. Use POST.",
			})
			return
		}

		var body struct {
			SensorType string `json:"sensor_type"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.SensorType == "" {
			writeJSON(w, http.StatusBadRequest, APIResponse{
				Status:  "error",
				Message: "Invalid JSON body. Provide 'sensor_type'.",
			})
			return
		}

		// TODO: Generate real unique tokens, persist to Redis/DB.
		token := fmt.Sprintf("tok_%d", time.Now().UnixNano())
		webhookURL := fmt.Sprintf("/api/iot/ingest/%s", body.SensorType)

		log.Printf("[iot-broker] Generated webhook for sensor type: %s", body.SensorType)

		writeJSON(w, http.StatusOK, APIResponse{
			Status:  "ok",
			Message: "Webhook provisioned.",
			Data: map[string]string{
				"sensor_type": body.SensorType,
				"webhook_url": webhookURL,
				"token":       token,
			},
		})
	})

	// POST /api/iot/ingest/:type — generic telemetry ingestion endpoint
	http.HandleFunc("/api/iot/ingest/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, APIResponse{
				Status:  "error",
				Message: "Method not allowed. Use POST.",
			})
			return
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeJSON(w, http.StatusBadRequest, APIResponse{
				Status:  "error",
				Message: "Invalid JSON payload.",
			})
			return
		}

		log.Printf("[iot-broker] Ingested telemetry: %v", payload)

		// TODO: Publish payload to RabbitMQ for downstream consumers.
		writeJSON(w, http.StatusAccepted, APIResponse{
			Status:  "ok",
			Message: "Telemetry accepted and queued.",
		})
	})

	log.Printf("iot-broker listening on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
