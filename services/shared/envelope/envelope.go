// Package envelope defines the Sūtra envelope: the one message schema used on
// RabbitMQ, the WebSocket fan-out and the REST mirror (CLAUDE.md §6).
package envelope

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Envelope struct {
	ID         string          `json:"id"`
	Entity     string          `json:"entity"`
	Type       string          `json:"type"`
	Geo        []float64       `json:"geo"`
	Status     *string         `json:"status"`
	Capacity   map[string]any  `json:"capacity"`
	Confidence float64         `json:"confidence"`
	Timestamp  string          `json:"timestamp"`
	Source     string          `json:"source"`
	Origin     string          `json:"origin"`
	Payload    json.RawMessage `json:"payload"`
}

// New builds an envelope. source names the producer ("simulated" or a SRC key where applicable);
// origin is the client or service id used to drop echoes.
func New(entity, typ, source, origin string) Envelope {
	return Envelope{
		ID: uuid.NewString(), Entity: entity, Type: typ, Confidence: 1,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Source: source, Origin: origin,
	}
}

func (e Envelope) Key() string { return e.Entity + "." + e.Type }

func (e Envelope) WithGeo(lat, lng float64) Envelope { e.Geo = []float64{lat, lng}; return e }
func (e Envelope) WithStatus(s string) Envelope      { e.Status = &s; return e }
func (e Envelope) WithCapacity(c map[string]any) Envelope {
	e.Capacity = c
	return e
}
func (e Envelope) WithConfidence(c float64) Envelope { e.Confidence = c; return e }

// WithPayload attaches the full domain object. A marshal failure leaves the payload empty.
func (e Envelope) WithPayload(v any) Envelope {
	if b, err := json.Marshal(v); err == nil {
		e.Payload = b
	}
	return e
}

// Decode unmarshals the payload into v.
func (e Envelope) Decode(v any) error {
	if len(e.Payload) == 0 {
		return json.Unmarshal([]byte("null"), v)
	}
	return json.Unmarshal(e.Payload, v)
}

// Fill supplies defaults for envelopes arriving from outside (the frontend mirror).
func (e *Envelope) Fill(origin string) {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	if e.Timestamp == "" {
		e.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if e.Origin == "" {
		e.Origin = origin
	}
	if e.Source == "" {
		e.Source = "unknown"
	}
}
