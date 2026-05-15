package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	_ = promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "wuzapi_sessions_connected",
		Help: "Number of currently connected whatsmeow sessions (logged in to WhatsApp).",
	}, func() float64 {
		clientManager.RLock()
		defer clientManager.RUnlock()
		n := 0
		for _, c := range clientManager.whatsmeowClients {
			if c != nil && c.IsConnected() && c.IsLoggedIn() {
				n++
			}
		}
		return float64(n)
	})

	_ = promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "wuzapi_sessions_total",
		Help: "Number of whatsmeow client instances tracked in memory.",
	}, func() float64 {
		clientManager.RLock()
		defer clientManager.RUnlock()
		return float64(len(clientManager.whatsmeowClients))
	})

	metricMessagesSentTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "wuzapi_messages_sent_total",
		Help: "Total messages sent through the API, by message type.",
	}, []string{"message_type"})

	metricMessagesReceivedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "wuzapi_messages_received_total",
		Help: "Total messages received from WhatsApp, by message type.",
	}, []string{"message_type"})

	metricWebhookDeliverySeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "wuzapi_webhook_delivery_seconds",
		Help:    "Webhook HTTP delivery duration, by outcome.",
		Buckets: prometheus.DefBuckets,
	}, []string{"outcome"})

	metricWebhookFailuresTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "wuzapi_webhook_failures_total",
		Help: "Total webhook deliveries that exhausted retries, by reason.",
	}, []string{"reason"})

	metricRabbitMQPublishTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "wuzapi_rabbitmq_publish_total",
		Help: "Total messages published to RabbitMQ, by queue and outcome.",
	}, []string{"queue", "outcome"})
)
