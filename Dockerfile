FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o metrics-analyzer

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/metrics-analyzer .
COPY --from=builder /app/kubeconfigs /app/kubeconfigs

EXPOSE 8080
CMD ["./metrics-analyzer"] 