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

	// NOTE: github.com/dillonstreator/opentelemetry-go-contrib/instrumentation/net/http/otelhttp is required
	// until https://github.com/open-telemetry/opentelemetry-go-contrib/pull/4591 is reviewed/merged
	// to ensure panic recovery logging includes request id and trace id
	"github.com/dillonstreator/opentelemetry-go-contrib/instrumentation/net/http/otelhttp"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/google/uuid"
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
		logger.Error("Setting up open telemetry", slog.Any("error", err))
		os.Exit(1)
	}

	mux := chi.NewMux()
	mux.Use(middleware.Recoverer)
	mux.Use(trustProxy(logger))
	mux.Use(otelhttp.NewMiddleware("chi"))
	mux.Use(func(h http.Handler) http.Handler {
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
			r.Body = http.MaxBytesReader(w, rc, cfg.maxAllowedRequestBytes)

			// overwrite `r`'s memory so that recoverer can access the log entry
			*r = *setLogger(r, l)
			*r = *middleware.WithLogEntry(r, newLogEntry(l))

			h.ServeHTTP(ww, r)

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

	mux.Get(cfg.healthEndpoint, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.Get("/hi", func(w http.ResponseWriter, r *http.Request) {
		l := getLogger(r)
		l.Info("hi")
		w.Write([]byte("hi"))
	})

	mux.Get("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("testing panic recovery and logging")
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.port),
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Server error", slog.Any("error", err))
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
		logger.Error("Server shutdown", slog.Any("error", err))
		os.Exit(1)
	}

	err = otelShutdown(ctx)
	if err != nil {
		logger.Error("Open telemetry shutdown", slog.Any("error", err))
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

func getLogger(r *http.Request) *slog.Logger {
	return r.Context().Value(ctxKeyLogger).(*slog.Logger)
}

func setLogger(r *http.Request, l *slog.Logger) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), ctxKeyLogger, l))
}

type logEntry struct {
	logger *slog.Logger
}

var _ middleware.LogEntry = (*logEntry)(nil)

func newLogEntry(logger *slog.Logger) *logEntry {
	return &logEntry{logger}
}

func (l *logEntry) Panic(v interface{}, stack []byte) {
	l.logger.Error("panic caught", slog.Any("panic", v), slog.String("stack", string(stack)))
}

func (l *logEntry) Write(status int, bytes int, header http.Header, elapsed time.Duration, extra interface{}) {
}
