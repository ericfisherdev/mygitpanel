package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

func main() {
	os.Exit(check())
}

func check() int {
	addr := normalizeAddr(os.Getenv("MYGITPANEL_LISTEN_ADDR"))

	client := &http.Client{Timeout: 2 * time.Second}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s/api/v1/health", addr), nil)
	if err != nil {
		return 1
	}

	resp, err := client.Do(req)
	if err != nil {
		return 1
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 1
	}

	return 0
}

// normalizeAddr ensures the healthcheck connects to loopback rather than the
// bind-all address. Docker containers bind 0.0.0.0 but the healthcheck runs
// inside the same container, so loopback is reachable and more correct.
func normalizeAddr(raw string) string {
	if raw == "" {
		return "127.0.0.1:8080"
	}

	host, port, err := net.SplitHostPort(raw)
	if err != nil {
		return "127.0.0.1:8080"
	}

	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}

	return net.JoinHostPort(host, port)
}
