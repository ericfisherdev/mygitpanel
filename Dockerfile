# Build stage
FROM golang:1.25-alpine AS build

# Install templ CLI
RUN go install github.com/a-h/templ/cmd/templ@latest

# Download Tailwind standalone CLI v4
RUN wget -O /usr/local/bin/tailwindcss \
    https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64-musl \
    && chmod +x /usr/local/bin/tailwindcss

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

# Generate templ Go files from .templ sources
RUN templ generate

# Build Tailwind CSS (must run after templ generate so _templ.go files exist for scanning)
RUN tailwindcss -i internal/adapter/driving/web/static/css/input.css \
    -o internal/adapter/driving/web/static/css/output.css --minify

# Build Go binaries (static assets embedded via go:embed including output.css)
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/mygitpanel ./cmd/mygitpanel
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/healthcheck ./cmd/healthcheck
RUN mkdir -p /data /tmp

# Runtime stage
FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /bin/mygitpanel /bin/mygitpanel
COPY --from=build /bin/healthcheck /bin/healthcheck
COPY --from=build /data /data
COPY --from=build /tmp /tmp

EXPOSE 8080
VOLUME /data

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD ["/bin/healthcheck"]

ENTRYPOINT ["/bin/mygitpanel"]
