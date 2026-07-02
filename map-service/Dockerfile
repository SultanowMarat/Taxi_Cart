FROM golang:1.23-alpine AS build
WORKDIR /app
COPY go.mod ./
COPY . .
RUN go build -o /map-service ./cmd/server

FROM alpine:3.21
WORKDIR /app
RUN mkdir -p /var/log/taxi /data/tiles /data/osm
COPY --from=build /map-service /app/map-service
EXPOSE 8090
CMD ["/app/map-service"]
