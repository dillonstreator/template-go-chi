package main

import (
	"io"
	"log/slog"
)

func newLogger(w io.Writer, lvl slog.Level) *slog.Logger {
	logger := slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: lvl,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Key = "ts"
				a.Value = slog.Int64Value(a.Value.Time().UnixNano())
			}
			if a.Key == slog.LevelKey {
				a.Key = "lvl"
			}

			return a
		},
	}))

	return logger
}
