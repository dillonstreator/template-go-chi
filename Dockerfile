FROM golang:1.25.12 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /server

FROM gcr.io/distroless/static-debian12:nonroot AS final

COPY --from=builder /server /server

ENV PORT=3000
EXPOSE 3000

USER nonroot:nonroot
ENTRYPOINT ["/server"]
