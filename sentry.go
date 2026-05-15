package main

import (
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/rs/zerolog"
)

// InitSentry initializes the Sentry/GlitchTip SDK when SENTRY_DSN is set.
// Returns true if Sentry is active. Caller should defer FlushSentry().
func InitSentry() bool {
	dsn := os.Getenv("SENTRY_DSN")
	if dsn == "" {
		return false
	}

	env := os.Getenv("SENTRY_ENVIRONMENT")
	if env == "" {
		env = "staging"
	}
	release := os.Getenv("SENTRY_RELEASE")

	if err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      env,
		Release:          release,
		TracesSampleRate: 0, // error tracking only, no performance traces
		AttachStacktrace: true,
	}); err != nil {
		return false
	}
	return true
}

// FlushSentry drains buffered events; call before process exit.
func FlushSentry() {
	sentry.Flush(2 * time.Second)
}

// sentryHook forwards zerolog Error/Fatal/Panic events to Sentry.
type sentryHook struct{}

func (sentryHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	if level < zerolog.ErrorLevel {
		return
	}
	sentry.CaptureMessage(msg)
}
