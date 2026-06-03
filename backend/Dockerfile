FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server

FROM alpine:3.21
RUN adduser -D -H -u 10001 appuser
WORKDIR /app
COPY --from=build /out/server /app/server
COPY migrations /app/migrations
USER appuser
EXPOSE 8080
ENTRYPOINT ["/app/server"]
