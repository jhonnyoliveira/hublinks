package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/hublinks/hublinks/internal/config"
)

// runHealthcheck consulta /healthz do próprio serviço (HTTP_ADDR) e falha se não
// responder 200. A imagem final não tem shell nem curl, então o healthcheck do
// compose usa o próprio binário.
func runHealthcheck(ctx context.Context, c config.Config) error {
	return checkHealth(ctx, healthURL(c.HTTPAddr))
}

// healthURL troca um endereço de escuta curinga (":8080", "0.0.0.0:8080") por loopback.
func healthURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://127.0.0.1:8080/healthz"
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port) + "/healthz"
}

func checkHealth(ctx context.Context, url string) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s respondeu %d", url, resp.StatusCode)
	}
	return nil
}
