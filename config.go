package main

import (
	"errors"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/docker/go-units"
)

type config struct {
	port                     int
	healthEndpoint           string
	logLevel                 slog.Level
	shutdownTimeout          time.Duration
	serviceName              string
	serviceVersion           string
	otelEnabled              bool
	otelExporterOTLPEndpoint *url.URL
	maxAllowedRequestBytes   int64
}

func newConfig() (*config, error) {
	errs := []error{}

	port, err := getEnv("PORT", strconv.Atoi, 3000)
	if err != nil {
		errs = append(errs, err)
	}

	healthEndpoint, err := getEnv("HEALTH_ENDPOINT", parseString, "/health")
	if err != nil {
		errs = append(errs, err)
	}

	logLevel, err := getEnv("LOG_LEVEL", parseLogLevel, slog.LevelInfo)
	if err != nil {
		errs = append(errs, err)
	}

	shutdownTimeout, err := getEnv("SHUTDOWN_TIMEOUT_DURATION", parseDuration, time.Second*15)
	if err != nil {
		errs = append(errs, err)
	}

	serviceName, err := getEnv("SERVICE_NAME", parseString, "go-chi")
	if err != nil {
		errs = append(errs, err)
	}

	serviceVersion, err := getEnv("SERVICE_VERSION", parseString, "v1.0.0")
	if err != nil {
		errs = append(errs, err)
	}

	otelEnabled, err := getEnv("OTEL_ENABLED", strconv.ParseBool, false)
	if err != nil {
		errs = append(errs, err)
	}

	otelExporterOTLPEndpoint, err := getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", url.Parse, nil)
	if err != nil {
		errs = append(errs, err)
	}

	maxAllowedRequestBytes, err := getEnv("MAX_ALLOWED_REQUEST_BYTES", units.FromHumanSize, int64(1000*1000*10))
	if err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return &config{
		port:                     port,
		healthEndpoint:           healthEndpoint,
		logLevel:                 logLevel,
		shutdownTimeout:          shutdownTimeout,
		serviceName:              serviceName,
		serviceVersion:           serviceVersion,
		otelEnabled:              otelEnabled,
		otelExporterOTLPEndpoint: otelExporterOTLPEndpoint,
		maxAllowedRequestBytes:   maxAllowedRequestBytes,
	}, nil
}

func getEnv[T any](key string, parser func(value string) (T, error), defaultValue T) (T, error) {
	value, ok := os.LookupEnv(key)
	if ok {
		parsed, err := parser(value)
		return parsed, errWrapf(err, "parsing env %s", key)
	}

	return defaultValue, nil
}

func parseLogLevel(value string) (slog.Level, error) {
	level := new(slog.LevelVar)
	err := level.UnmarshalText([]byte(value))
	if err != nil {
		return 0, err
	}

	return level.Level(), nil
}

func parseDuration(value string) (time.Duration, error) {
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, err
	}

	return duration, nil
}

func parseString(value string) (string, error) {
	return value, nil
}
