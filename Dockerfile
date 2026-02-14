# Build stage
FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/mygitpanel ./cmd/mygitpanel
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/healthcheck ./cmd/healthcheck
RUN mkdir -p /data /tmp

# Runtime stage
FROM scratch

COPY --from=build /bin/mygitpanel /bin/mygitpanel
COPY --from=build /bin/healthcheck /bin/healthcheck
COPY --from=build /data /data
COPY --from=build /tmp /tmp

EXPOSE 8080
VOLUME /data

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD ["/bin/healthcheck"]

ENTRYPOINT ["/bin/mygitpanel"]
