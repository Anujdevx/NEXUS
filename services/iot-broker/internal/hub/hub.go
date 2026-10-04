// Package hub fans envelopes out to connected WebSocket clients, filtered by role (CLAUDE.md §6).
package hub

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"nexus/shared/auth"
	"nexus/shared/envelope"
)

type Client struct {
	ID   string // the client id sent as ?client=; envelopes with this origin are never echoed back
	Role string
	Unit string // optional: a responder's unit id
	send chan []byte
}

type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]struct{}
	log     *slog.Logger
}

func New(log *slog.Logger) *Hub { return &Hub{clients: map[*Client]struct{}{}, log: log} }

func (h *Hub) Count() int { h.mu.RLock(); defer h.mu.RUnlock(); return len(h.clients) }

// wants applies the role filter and drops echoes.
func (c *Client) wants(env envelope.Envelope) bool {
	if env.Origin != "" && env.Origin == c.ID {
		return false
	}
	switch c.Role {
	case auth.Controller:
		return true
	case auth.Responder:
		switch env.Entity {
		case "road_segment", "public_alert", "route":
			return true
		case "assignment", "unit":
			if c.Unit == "" {
				return true
			}
			var p struct {
				Unit string `json:"unit"`
				ID   string `json:"id"`
			}
			_ = json.Unmarshal(env.Payload, &p)
			return p.Unit == c.Unit || p.ID == c.Unit
		}
		return false
	default: // Citizen
		if env.Entity == "public_alert" {
			return true
		}
		if env.Entity == "sos" || env.Entity == "assignment" {
			var p struct {
				Client string `json:"client"`
			}
			_ = json.Unmarshal(env.Payload, &p)
			return p.Client != "" && p.Client == c.ID
		}
		return false
	}
}

// Broadcast sends the envelope to every client that wants it. Slow clients are dropped, not waited on.
func (h *Hub) Broadcast(env envelope.Envelope) {
	body, err := json.Marshal(env)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if !c.wants(env) {
			continue
		}
		select {
		case c.send <- body:
		default:
			h.log.Warn("ws client too slow, message dropped", "client", c.ID)
		}
	}
}

// Serve runs one connection until it closes.
func (h *Hub) Serve(ctx context.Context, conn *websocket.Conn, c *Client) {
	c.send = make(chan []byte, 256)
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	h.log.Info("ws connected", "client", c.ID, "role", c.Role, "clients", h.Count())
	defer func() {
		h.mu.Lock()
		delete(h.clients, c)
		h.mu.Unlock()
		conn.CloseNow()
		h.log.Info("ws closed", "client", c.ID, "clients", h.Count())
	}()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// reader: the console only listens, but this notices closes and answers pings
	go func() {
		defer cancel()
		for {
			if _, _, err := conn.Read(ctx); err != nil {
				return
			}
		}
	}()
	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case b := <-c.send:
			wctx, wc := context.WithTimeout(ctx, 5*time.Second)
			err := conn.Write(wctx, websocket.MessageText, b)
			wc()
			if err != nil {
				return
			}
		case <-ping.C:
			pctx, pc := context.WithTimeout(ctx, 5*time.Second)
			err := conn.Ping(pctx)
			pc()
			if err != nil {
				return
			}
		}
	}
}

// Roles used to describe a client for logs.
func Describe(c *Client) string { return strings.ToLower(c.Role) + ":" + c.ID }
