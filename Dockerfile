FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /support-agent .

FROM alpine:3.20

WORKDIR /app

COPY --from=builder /support-agent ./support-agent
COPY config.json.example response_message.json ./

EXPOSE 8080

ENTRYPOINT ["./support-agent"]
