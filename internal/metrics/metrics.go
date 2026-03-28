package metrics

import (
	"net/http"
	"strconv"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests",
	}, []string{"method", "path", "status"})

	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	DNOQueryTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "dno_query_total",
		Help: "Total DNO queries",
	}, []string{"channel", "result"})

	DNOQueryDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "dno_query_duration_seconds",
		Help:    "DNO query duration in seconds",
		Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5},
	})

	BulkQuerySize = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "dno_bulk_query_size",
		Help:    "Number of phone numbers per bulk query",
		Buckets: []float64{1, 10, 50, 100, 250, 500, 1000},
	})

	QueryLogBufferSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "querylog_buffer_size",
		Help: "Current size of the async query log buffer",
	})

	CacheHits = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cache_hits_total",
		Help: "Cache hit count",
	}, []string{"cache"})

	CacheMisses = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cache_misses_total",
		Help: "Cache miss count",
	}, []string{"cache"})
)

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		next.ServeHTTP(ww, r)
		duration := time.Since(start).Seconds()

		HTTPRequestsTotal.WithLabelValues(r.Method, r.URL.Path, strconv.Itoa(ww.Status())).Inc()
		HTTPRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
	})
}
