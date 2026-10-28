FROM golang:1.27-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o api ./cmd/api && \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o create-admin ./cmd/create-admin && \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o create-bin ./cmd/create-bin

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/api /app/create-admin /app/create-bin ./

ENV GIN_MODE=release

EXPOSE 8080

CMD ["./api"]
