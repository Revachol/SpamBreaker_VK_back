package httpmetric

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var _ HttpMetricsIface = (*PrometheusHttpCollector)(nil)

type PrometheusHttpCollector struct {
	serviceName string
	registry    *prometheus.Registry

	httpRequests *prometheus.CounterVec
	httpDuration *prometheus.HistogramVec
}

func NewPrometheusHttpCollector(serviceName string, reg *prometheus.Registry) *PrometheusHttpCollector {

	mtrc := &PrometheusHttpCollector{
		serviceName: serviceName,
		registry:    reg,

		httpRequests: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status", "service"},
		),
		httpDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "Duration of HTTP requests",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
			[]string{"method", "path", "status", "service"},
		),
	}

	// Регистрируем только наши кастомные метрики
	reg.MustRegister(mtrc.httpRequests)
	reg.MustRegister(mtrc.httpDuration)

	return mtrc
}

func (p *PrometheusHttpCollector) IncHTTPRequest(method, path string, statusCode int) {
	p.httpRequests.WithLabelValues(method, path, strconv.Itoa(statusCode), p.serviceName).Inc()
}

func (p *PrometheusHttpCollector) ObserveHTTPDuration(method, path string, statusCode int, duration float64) {
	p.httpDuration.WithLabelValues(method, path, strconv.Itoa(statusCode), p.serviceName).Observe(duration)
}
