package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
)

func main() {
	cfg, err := newConfig()
	if err != nil {
		log.Fatal(err)
	}

	logger := newLogger(os.Stdout, cfg.logLevel)

	otelShutdown, err := setupOTelSDK(context.Background(), cfg)
	if err != nil {
		logger.Error("Setting up open telemetry", "err", err)
		os.Exit(1)
	}

	r := chi.NewMux()

	r.Use(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			traceID := trace.SpanFromContext(r.Context()).SpanContext().TraceID()
			var reqID string

			if id, err := uuid.Parse(r.Header.Get("x-request-id")); err == nil {
				reqID = id.String()
			} else {
				reqID = uuid.NewString()
			}

			l := logger.With("reqId", reqID, "traceId", traceID)

			ww := middleware.NewWrapResponseWriter(w, 0)
			rc := newByteReadCloser(r.Body)
			r.Body = rc
			h.ServeHTTP(ww, setLoggerReq(r, l))

			l.Info(
				"Request handled",
				slog.String("method", r.Method),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("ua", r.UserAgent()),
				slog.String("ip", r.RemoteAddr),
				slog.Int("bw", ww.BytesWritten()),
				slog.Int64("br", rc.BytesRead()),
				slog.Int("status", ww.Status()),
				slog.Duration("duration", time.Since(start)),
			)
		})
	})

	r.Get(cfg.healthEndpoint, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r.Get("/hi", func(w http.ResponseWriter, r *http.Request) {
		l := getLoggerReq(r)
		l.Info("hi")
		w.Write([]byte("hi"))
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.port),
		Handler: otelhttp.NewHandler(r, "chi"),
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Server error", "err", err)
			os.Exit(1)
		}
	}()

	logger.Info(fmt.Sprintf("Listening for HTTP on port %d", cfg.port))

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	sig := <-shutdown
	logger.Info("Shutdown signal received", "signal", sig.String())

	ctx, cancel := context.WithTimeout(context.Background(), cfg.shutdownTimeout)
	defer cancel()

	err = srv.Shutdown(ctx)
	if err != nil {
		logger.Error("Server shutdown", "err", err)
		os.Exit(1)
	}

	err = otelShutdown(ctx)
	if err != nil {
		logger.Error("Open telemetry shutdown", "err", err)
		os.Exit(1)
	}
}

type byteReadCloser struct {
	rc io.ReadCloser
	n  int64
}

func newByteReadCloser(r io.ReadCloser) *byteReadCloser {
	return &byteReadCloser{r, 0}
}

func (br *byteReadCloser) Read(p []byte) (int, error) {
	n, err := br.rc.Read(p)
	br.n += int64(n)
	return n, err
}

func (br *byteReadCloser) Close() error {
	return br.rc.Close()
}

func (br *byteReadCloser) BytesRead() int64 {
	return br.n
}

type ctxKey string

const (
	ctxKeyLogger ctxKey = "logger"
)

func getLoggerReq(r *http.Request) *slog.Logger {
	return r.Context().Value(ctxKeyLogger).(*slog.Logger)
}

func setLoggerReq(r *http.Request, l *slog.Logger) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), ctxKeyLogger, l))
}
