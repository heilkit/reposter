FROM golang:1.22-alpine3.19 AS env

WORKDIR /app
COPY go.mod go.sum main.go /app/
RUN go build -o reposter


FROM alpine:3.19

WORKDIR /app
COPY --from=env /app/reposter /app/

ENTRYPOINT ./reposter -config ./data/config.json