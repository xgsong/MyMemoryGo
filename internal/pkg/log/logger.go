// Package log provides structured logging utilities.
package log

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/samber/slog-formatter"
	"github.com/samber/slog-multi"
	sampling "github.com/samber/slog-sampling"
)

var defaultLogger *slog.Logger

func init() {
	defaultLogger = NewLogger()
	slog.SetDefault(defaultLogger)
}

type LoggerConfig struct {
	Level       slog.Level
	JSONFormat  bool
	ServiceName string
}

func NewLogger() *slog.Logger {
	cfg := LoggerConfig{
		Level:       slog.LevelInfo,
		JSONFormat:  true,
		ServiceName: "go-memory",
	}
	return NewLoggerWithConfig(cfg)
}

func NewLoggerWithConfig(cfg LoggerConfig) *slog.Logger {
	handlerOpts := &slog.HandlerOptions{
		Level: cfg.Level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == "time" {
				if t, ok := a.Value.Any().(time.Time); ok {
					a.Value = slog.StringValue(t.Format(time.RFC3339))
				}
			}
			return a
		},
	}

	var baseHandler slog.Handler
	if cfg.JSONFormat {
		baseHandler = slog.NewJSONHandler(os.Stdout, handlerOpts)
	} else {
		baseHandler = slog.NewTextHandler(os.Stdout, handlerOpts)
	}

	pipe := slogmulti.Pipe(
		sampling.ThresholdSamplingOption{
			Tick:      5 * time.Second,
			Threshold: 10,
			Rate:      0.1,
		}.NewMiddleware(),
		slogformatter.NewFormatterMiddleware(
			slogformatter.ErrorFormatter("error"),
		),
	).Handler(baseHandler)

	return slog.New(pipe).With("service", cfg.ServiceName)
}

func DefaultLogger() *slog.Logger {
	return defaultLogger
}

func SetDefaultLogger(logger *slog.Logger) {
	defaultLogger = logger
}

type logContextKey struct{}

func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, logContextKey{}, logger)
}

func GetLogger(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(logContextKey{}).(*slog.Logger); ok {
		return logger
	}
	return defaultLogger
}
