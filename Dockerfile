# Build stage
FROM golang:1.20.6-alpine3.17 AS builder
WORKDIR /app
COPY . .
RUN go build -o main main.go

# Run stage
FROM alpine:3.14
WORKDIR /app
COPY --from=builder /app/main .
COPY .env .

EXPOSE 8000
CMD [ "/app/main" ]