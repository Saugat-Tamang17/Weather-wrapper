FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o weather-wrapper ./main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/weather-wrapper .

EXPOSE 8080

CMD ["./weather-wrapper"]

