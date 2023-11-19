FROM golang:1.21 as builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o /server

FROM gcr.io/distroless/base-debian11 as final

COPY --from=builder /server /server

ENV PORT 3000
EXPOSE $PORT

ENTRYPOINT ["/server"]
