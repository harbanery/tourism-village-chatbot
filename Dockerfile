FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /tourism-village-chatbot .

FROM alpine:3.20

WORKDIR /app

COPY --from=builder /tourism-village-chatbot ./tourism-village-chatbot
COPY config.json.example response_message.json ./

EXPOSE 8080

ENTRYPOINT ["./tourism-village-chatbot"]
