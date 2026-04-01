package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
)

var (
	log     *slog.Logger
	handler slog.Handler
)

func Init(level slog.Level) {
	handler = tint.NewHandler(os.Stderr, &tint.Options{
		Level:      level,
		TimeFormat: "15:04:05",
	})
	log = slog.New(handler)
}

func Get() *slog.Logger {
	return log
}

func Debug(msg string, args ...any) {
	log.Log(context.Background(), slog.LevelDebug, msg, args...)
}

func Debugf(msg string, args ...any) {
	log.Log(context.Background(), slog.LevelDebug, fmt.Sprintf(msg, args...))
}

func Info(msg string, args ...any) {
	log.Log(context.Background(), slog.LevelInfo, msg, args...)
}

func Infof(msg string, args ...any) {
	log.Log(context.Background(), slog.LevelInfo, fmt.Sprintf(msg, args...))
}

func Warn(msg string, args ...any) {
	log.Log(context.Background(), slog.LevelWarn, msg, args...)
}

func Warnf(msg string, args ...any) {
	log.Log(context.Background(), slog.LevelWarn, fmt.Sprintf(msg, args...))
}

func Error(msg string, args ...any) {
	log.Log(context.Background(), slog.LevelError, msg, args...)
}

func Errorf(msg string, args ...any) {
	log.Log(context.Background(), slog.LevelError, fmt.Sprintf(msg, args...))
}

func Fatal(msg string, args ...any) {
	Error(msg, args...)
	os.Exit(1)
}

func Fatalf(msg string, args ...any) {
	Errorf(msg, args...)
	os.Exit(1)
}

func With(args ...any) *slog.Logger {
	return Get().With(args...)
}
