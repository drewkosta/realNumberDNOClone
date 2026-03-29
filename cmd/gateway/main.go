package main

import (
	"context"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"realNumberDNOClone/internal/metrics"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	gatewayPort := envOr("GATEWAY_PORT", "8080")
	queryURL := envOr("QUERY_SERVICE_URL", "http://localhost:8081")
	portalURL := envOr("PORTAL_SERVICE_URL", "http://localhost:8082")
	staticDir := envOr("STATIC_DIR", "") // e.g., ./client/dist

	queryTarget, _ := url.Parse(queryURL)
	portalTarget, _ := url.Parse(portalURL)

	queryProxy := retryProxy(queryTarget, logger)
	portalProxy := retryProxy(portalTarget, logger)

	r := chi.NewRouter()
	r.Use(chimw.RequestID, chimw.RealIP, chimw.Recoverer, metrics.Middleware)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-API-Key"},
		ExposedHeaders:   []string{"Content-Disposition", "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset", "Retry-After"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Handle("/metrics", promhttp.Handler())
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"gateway"}`))
	})
	r.Get("/ready", func(w http.ResponseWriter, _ *http.Request) {
		// Gateway is ready if it can reach both backends
		qOK := checkBackend(queryURL + "/health")
		pOK := checkBackend(portalURL + "/health")
		if !qOK || !pOK {
			w.WriteHeader(http.StatusServiceUnavailable)
			writeJSON(w, map[string]interface{}{"ready": false, "query": qOK, "portal": pOK})
			return
		}
		writeJSON(w, map[string]interface{}{"ready": true})
	})

	// Route query endpoints to query-service
	r.Get("/api/v1/dno/query", queryProxy.ServeHTTP)
	r.Post("/api/v1/dno/query/bulk", queryProxy.ServeHTTP)

	// Route all other API calls to portal-service
	r.HandleFunc("/api/v1/*", portalProxy.ServeHTTP)

	// Serve static frontend files in production
	if staticDir != "" {
		serveSPA(r, staticDir, logger)
	}

	logger.Info("Gateway starting", "port", gatewayPort, "query", queryURL, "portal", portalURL, "static", staticDir)

	srv := &http.Server{
		Addr:              ":" + gatewayPort,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Gateway failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("Gateway shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
	logger.Info("Gateway stopped")
}

// retryProxy wraps a reverse proxy with a single retry on connection errors.
func retryProxy(target *url.URL, logger *slog.Logger) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(target)
	defaultTransport := http.DefaultTransport
	proxy.Transport = &retryTransport{
		inner:  defaultTransport,
		logger: logger,
		target: target.Host,
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		logger.Error("proxy error", "target", target.Host, "path", r.URL.Path, "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"error":"service unavailable"}`))
	}
	return proxy
}

type retryTransport struct {
	inner  http.RoundTripper
	logger *slog.Logger
	target string
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.inner.RoundTrip(req)
	if err != nil {
		// Retry once on connection error
		t.logger.Warn("proxy: retrying", "target", t.target, "path", req.URL.Path, "error", err)
		time.Sleep(100 * time.Millisecond)
		return t.inner.RoundTrip(req)
	}
	return resp, nil
}

// serveSPA serves the frontend SPA from a directory, falling back to index.html
// for client-side routing.
func serveSPA(r chi.Router, dir string, logger *slog.Logger) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		logger.Error("invalid static dir", "dir", dir, "error", err)
		return
	}

	fsys := http.Dir(absDir)
	fileServer := http.FileServer(fsys)

	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		// Try to serve the file directly
		if _, err := fs.Stat(os.DirFS(absDir), path); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		// Fall back to index.html for SPA routing
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})

	logger.Info("Serving static SPA", "dir", absDir)
}

func checkBackend(url string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
