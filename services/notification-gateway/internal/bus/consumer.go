// Package bus raises automatic alerts from the bus: infrastructure cascades and severe hazard risk,
// and records alerts the console issues itself. SMS is a stub: it publishes telecom.connectivity and logs.
package bus

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"nexus/shared/envelope"

	"github.com/nexus/notification-gateway/internal/domain"
	api "github.com/nexus/notification-gateway/internal/http"
	"github.com/nexus/notification-gateway/internal/store"
)

var Keys = []string{"cascade.computed", "hazard.risk", "assignment.dispatched", "public_alert.cap_alert"}

type Consumer struct {
	API   *api.API
	Store *store.Store
	Log   *slog.Logger

	mu       sync.Mutex
	lastRisk map[string]time.Time
}

func New(a *api.API, s *store.Store, log *slog.Logger) *Consumer {
	return &Consumer{API: a, Store: s, Log: log, lastRisk: map[string]time.Time{}}
}

func (c *Consumer) Handle(ctx context.Context, env envelope.Envelope) error {
	switch env.Key() {
	case "cascade.computed":
		var p struct {
			Root     string   `json:"root"`
			RootName string   `json:"rootName"`
			RootType string   `json:"rootType"`
			Failed   []string `json:"failed"`
		}
		if env.Decode(&p) != nil || p.Root == "" {
			return nil
		}
		area := p.RootName
		if area == "" {
			area = p.Root
		}
		en, hi := domain.InfraText(p.RootType, area)
		al, err := domain.NewAlert(area, "Severe", en, hi, "system", "auto: cascade", "Infrastructure failure")
		if err != nil {
			return err
		}
		if err := c.API.Publish(ctx, al); err != nil {
			return err
		}
		if p.RootType == "TELECOM" {
			_ = c.API.Pub.Publish(ctx, envelope.New("telecom", "connectivity", "simulated", "notification-gateway").WithStatus("down").
				WithPayload(map[string]any{"tower": p.Root, "area": area}))
		}
	case "hazard.risk":
		var p struct {
			Level string  `json:"level"`
			MM    float64 `json:"mm"`
			Basis string  `json:"basis"`
		}
		// a what-if is a rehearsal, never a public alert
		if env.Decode(&p) != nil || p.Basis == "what-if" || (p.Level != "severe" && p.Level != "high") {
			return nil
		}
		c.mu.Lock()
		last := c.lastRisk[p.Level]
		fresh := time.Since(last) > 2*time.Minute
		if fresh {
			c.lastRisk[p.Level] = time.Now()
		}
		c.mu.Unlock()
		if !fresh {
			return nil
		}
		sev := "Severe"
		if p.Level == "severe" {
			sev = "Extreme"
		}
		al, err := domain.NewAlert("Dehradun district", sev, domain.DefaultEN, domain.DefaultHI, "system", "auto: hazard risk", "Flood")
		if err != nil {
			return err
		}
		return c.API.Publish(ctx, al)
	case "assignment.dispatched":
		var p struct {
			Via    string `json:"via"`
			Client string `json:"client"`
			Hosp   struct {
				N string `json:"n"`
			} `json:"hospital"`
		}
		if env.Decode(&p) == nil && (p.Via == "SMS gateway" || p.Via == "peer relay") {
			// SMS fallback stub: no message leaves the building.
			c.Log.Info("sms fallback (stub)", "via", p.Via, "text", fmt.Sprintf("Help is assigned. Go to %s.", p.Hosp.N))
			_ = c.API.Pub.Publish(ctx, envelope.New("telecom", "connectivity", "stub", "notification-gateway").WithStatus("sms_stub").
				WithPayload(map[string]any{"via": p.Via, "client": p.Client, "text": fmt.Sprintf("Help is assigned. Go to %s.", p.Hosp.N), "stub": true}))
		}
	case "public_alert.cap_alert":
		// an alert issued from the console: keep the server-side record (not re-published)
		var p struct {
			ID       string `json:"id"`
			Area     string `json:"area"`
			Severity string `json:"severity"`
			Message  string `json:"message"`
			By       string `json:"by"`
		}
		if env.Decode(&p) != nil || p.Area == "" {
			return nil
		}
		al, err := domain.NewAlert(p.Area, p.Severity, p.Message, "", p.By, "console", "")
		if err != nil {
			return nil
		}
		return c.Store.Save(ctx, al)
	}
	return nil
}
