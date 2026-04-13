package botmetrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var _ BotMetricsIface = (*PrometheusBotCollector)(nil)

type PrometheusBotCollector struct {
	serviceName string
	registry    *prometheus.Registry

	botRequests *prometheus.CounterVec
	botDuration *prometheus.HistogramVec
}

func NewPrometheusBotCollector(serviceName string, reg *prometheus.Registry) *PrometheusBotCollector {

	mtrc := &PrometheusBotCollector{
		serviceName: serviceName,
		registry:    reg,

		botRequests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "requests_total",
				Help: "Total number of bot requests",
			},
			[]string{"bot", "command", "status"}, // botName добавлен в лейблы
		),

		botDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "request_duration_seconds",
				Help:    "Duration of bot requests",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"bot", "command", "status"},
		),
	}

	// Регистрируем только наши кастомные метрики
	reg.MustRegister(mtrc.botRequests)
	reg.MustRegister(mtrc.botDuration)

	return mtrc
}

func (p *PrometheusBotCollector) IncBotRequest(command, status string) {
	p.botRequests.WithLabelValues(p.serviceName, command, status).Inc()
}

func (p *PrometheusBotCollector) ObserveBotDuration(command, status string, duration float64) {
	p.botDuration.WithLabelValues(p.serviceName, command, status).Observe(duration)
}
