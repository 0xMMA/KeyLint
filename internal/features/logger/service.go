// Package logger exposes a Wails-registered service so the Angular frontend
// can forward log messages into the Go debug.log file.
package logger

import (
	"context"

	"keylint/internal/logger"
)

// Service forwards frontend log messages into the Go structured logger.
type Service struct{}

// NewService creates a new LogService.
func NewService() *Service { return &Service{} }

// Log writes a frontend message at the given level into debug.log.
// The msg is wrapped in Redact() because frontend messages may contain user text.
// Error and warn levels log the msg directly (operational).
func (s *Service) Log(level, msg string) {
	fl := logger.FrontendLogger()
	switch level {
	case "trace":
		fl.Log(context.Background(), logger.LevelTrace, "frontend", "msg", logger.Redact(msg))
	case "debug":
		fl.Debug("frontend", "msg", logger.Redact(msg))
	case "warn":
		fl.Warn("frontend", "msg", msg)
	case "error":
		fl.Error("frontend", "msg", msg)
	default:
		fl.Info("frontend", "msg", logger.Redact(msg))
	}
}
