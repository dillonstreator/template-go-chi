# template-go-chi

A minimal production-ready golang template for an HTTP server with [`go-chi/chi`](https://github.com/go-chi/chi).

✅ Graceful shutdown \
✅ Tracing with OpenTelemetry \
✅ Logging with `log/slog` \
✅ Rich request logging middleware including bytes written/read, request id, trace id, context propagation, and more

## Installation

Go 1.20+ required

```sh
git clone https://github.com/dillonstreator/template-go-chi

cd template-go-chi

go run .
```

## Configuration

See all example configuration via environment variables in [`.env-example`](./.env-example)

### Open Telemetry

Open Telemetry is disabled by default but can be enabled by setting the `OTEL_ENABLED` environment to `true`.

By default, the trace exporter is set to standard output. This can be overridden by setting `OTEL_EXPORTER_OTLP_ENDPOINT`.

Start the `jaegertracing/all-in-one` container with `docker-compose up` and set `OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318` to collect logs in jaeger. Docker compose will expose jaeger at http://localhost:16686
