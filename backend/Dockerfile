FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/api
# Migrations stay in the builder stage so the runtime layer can copy them.


FROM alpine:latest

RUN apk --no-cache add ca-certificates curl

WORKDIR /root/

COPY --from=builder /app/main .
COPY --from=builder /app/migrations /app/migrations

EXPOSE 8080

ENV MIGRATIONS_DIR=/app/migrations

CMD ["./main"]
