package logger

import (
	"context"
	"fmt"
	"log/slog"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
)

type LarkLogger struct {
	log   *slog.Logger
	level slog.Level
}

func NewLarkLogger(level slog.Level) *LarkLogger {
	return &LarkLogger{
		log:   slog.Default().With(slog.String("lib", "lark")),
		level: level,
	}
}

func (l *LarkLogger) Debug(ctx context.Context, args ...interface{}) {
	if slog.LevelDebug >= l.level {
		log.Log(ctx, slog.LevelDebug, fmt.Sprint(args...))
	}
}

func (l *LarkLogger) Info(ctx context.Context, args ...interface{}) {
	if slog.LevelInfo >= l.level {
		log.Log(ctx, slog.LevelInfo, fmt.Sprint(args...))
	}
}

func (l *LarkLogger) Warn(ctx context.Context, args ...interface{}) {
	if slog.LevelWarn >= l.level {
		log.Log(ctx, slog.LevelWarn, fmt.Sprint(args...))
	}
}

func (l *LarkLogger) Error(ctx context.Context, args ...interface{}) {
	if slog.LevelError >= l.level {
		log.Log(ctx, slog.LevelError, fmt.Sprint(args...))
	}
}

var _ larkcore.Logger = (*LarkLogger)(nil)
