package common

import (
	"log/slog"
)

func NewLogger() *slog.Logger {
	slog.SetLogLoggerLevel(slog.LevelInfo)
	return slog.New(slog.Default().Handler())
}
