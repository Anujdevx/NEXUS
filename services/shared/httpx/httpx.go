// Package httpx holds the HTTP plumbing every service shares: routing on the Go 1.22
// pattern mux, JSON helpers, the error shape, CORS, request ids, panic recovery,
// structured access logs, /healthz, /readyz and graceful shutdown.
package httpx

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type App struct {
	Name string
	Log  *slog.Logger
	Mux  *http.ServeMux

	mu      sync.Mutex
	checks  map[string]func(context.Context) error
	order   []string
	onStop  []func(context.Context)
	started time.Time
}

// New creates the service shell with JSON logging to stdout.
func New(name string) *App {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", name)
	slog.SetDefault(log)
	a := &App{Name: name, Log: log, Mux: http.NewServeMux(), checks: map[string]func(context.Context) error{}, started: time.Now()}
	a.Mux.HandleFunc("GET /healthz", a.healthz)
	a.Mux.HandleFunc("GET /readyz", a.readyz)
	// reachable through the gateway at /svc/<service>/healthz
	a.Mux.HandleFunc("GET /svc/"+name+"/healthz", a.healthz)
	a.Mux.HandleFunc("GET /svc/"+name+"/readyz", a.readyz)
	return a
}

// Ready registers a dependency probe used by /readyz.
func (a *App) Ready(name string, check func(context.Context) error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.checks[name]; !ok {
		a.order = append(a.order, name)
	}
	a.checks[name] = check
}

// OnStop registers cleanup that runs after the server has drained.
func (a *App) OnStop(f func(context.Context)) { a.onStop = append(a.onStop, f) }

func (a *App) Handle(pattern string, h http.HandlerFunc) { a.Mux.HandleFunc(pattern, h) }

func (a *App) healthz(w http.ResponseWriter, _ *http.Request) {
	JSON(w, 200, map[string]any{"status": "ok", "service": a.Name, "uptime_s": int(time.Since(a.started).Seconds())})
}

func (a *App) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	a.mu.Lock()
	order := append([]string(nil), a.order...)
	checks := map[string]func(context.Context) error{}
	for k, v := range a.checks {
		checks[k] = v
	}
	a.mu.Unlock()
	res := map[string]string{}
	ok := true
	for _, n := range order {
		if err := checks[n](ctx); err != nil {
			res[n] = err.Error()
			ok = false
		} else {
			res[n] = "ok"
		}
	}
	code, st := 200, "ready"
	if !ok {
		code, st = 503, "not ready"
	}
	JSON(w, code, map[string]any{"status": st, "service": a.Name, "dependencies": res})
}

// Run serves on :PORT (default 8080) until SIGTERM/SIGINT, then drains gracefully.
func (a *App) Run(ctx context.Context) error {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{Addr: ":" + port, Handler: a.chain(a.Mux), ReadHeaderTimeout: 10 * time.Second}
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	errc := make(chan error, 1)
	go func() {
		a.Log.Info("listening", "addr", srv.Addr)
		errc <- srv.ListenAndServe()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	a.Log.Info("shutting down")
	sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := srv.Shutdown(sctx)
	for _, f := range a.onStop {
		f(sctx)
	}
	return err
}

func (a *App) chain(h http.Handler) http.Handler {
	return cors(requestID(recoverer(a.Log, accessLog(a.Log, h))))
}

// ---- JSON helpers ----

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Items writes the list shape {"items": [...]}.
func Items(w http.ResponseWriter, items any) {
	if items == nil {
		items = []any{}
	}
	JSON(w, 200, map[string]any{"items": items})
}

type errBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func Error(w http.ResponseWriter, status int, code, msg string) {
	var b errBody
	b.Error.Code, b.Error.Message = code, msg
	JSON(w, status, b)
}

// Decode reads a JSON body (1 MiB cap). It writes the 400 itself and returns false on failure.
func Decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := dec.Decode(v); err != nil && !errors.Is(err, io.EOF) {
		Error(w, http.StatusBadRequest, "bad_request", "invalid JSON body: "+err.Error())
		return false
	}
	return true
}

// ClientIP is the best-effort caller address (Traefik sets X-Forwarded-For).
func ClientIP(r *http.Request) string {
	if f := r.Header.Get("X-Forwarded-For"); f != "" {
		for i := 0; i < len(f); i++ {
			if f[i] == ',' {
				return f[:i]
			}
		}
		return f
	}
	h, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return h
}

// ---- middleware ----

type ridKey struct{}

func RequestID(ctx context.Context) string { s, _ := ctx.Value(ridKey{}).(string); return s }

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			b := make([]byte, 8)
			_, _ = rand.Read(b)
			id = hex.EncodeToString(b)
		}
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ridKey{}, id)))
	})
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Service-Key, X-Request-Id")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func recoverer(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error("panic", "path", r.URL.Path, "recover", fmt.Sprint(rec))
				Error(w, http.StatusInternalServerError, "internal", "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type recorder struct {
	http.ResponseWriter
	status int
}

func (r *recorder) WriteHeader(c int)           { r.status = c; r.ResponseWriter.WriteHeader(c) }
func (r *recorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }
func (r *recorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
func (r *recorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("hijack unsupported")
	}
	r.status = http.StatusSwitchingProtocols
	return h.Hijack()
}

func accessLog(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &recorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rec, r)
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			return
		}
		log.Info("http", "method", r.Method, "path", r.URL.Path, "status", rec.status, "ms", time.Since(start).Milliseconds(), "rid", RequestID(r.Context()))
	})
}
