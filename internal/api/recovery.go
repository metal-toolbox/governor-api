package api

import (
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// safeLogger wraps *zap.Logger and implements ginzap.ZapLogger.
// It drops the raw "request" field from panic recovery log entries to prevent
// credential leakage (e.g. Authorization / Cookie headers).
type safeLogger struct {
	*zap.Logger
}

// Error overrides *zap.Logger.Error, stripping any "request" field before logging.
func (s safeLogger) Error(msg string, fields ...zap.Field) {
	filtered := fields[:0:len(fields)]

	for _, f := range fields {
		if f.Key == "request" && f.Type == zapcore.StringType {
			continue
		}

		filtered = append(filtered, f)
	}

	s.Logger.Error(msg, filtered...)
}

// recoveryMiddleware returns a gin.HandlerFunc that uses ginzap.RecoveryWithZap
// with a safeLogger so the raw HTTP request dump (which includes headers) is
// never written to the log when a panic occurs.
func recoveryMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return ginzap.RecoveryWithZap(safeLogger{logger}, true)
}
