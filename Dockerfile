# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata \
	&& adduser -D -H -u 10001 weles
WORKDIR /app
COPY --from=build /out/server /app/server
COPY config/apps.example.yaml /app/config/apps.yaml
USER weles
# WARNING: skip + stdout are for local/dev only. Public deploys must use
# WELLS_TURNSTILE_MODE=cloudflare and WELLS_TURNSTILE_SSM (or SECRET) + SES.
ENV WELLS_APPS_CONFIG=/app/config/apps.yaml \
	WELLS_LISTEN_ADDR=:8080 \
	WELLS_TURNSTILE_MODE=skip \
	WELLS_NOTIFIER=stdout
EXPOSE 8080
ENTRYPOINT ["/app/server"]
