package main

import (
	"context"
	"errors"
	"fmt"
	"intermediate/internal/limiter"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func createProxyHandler(target string) (http.Handler, error) {
	targetURL, err := url.Parse(target)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		if errors.Is(err, context.DeadlineExceeded) {
			http.Error(w, "504 Gateway Timeout - The backend took too long!", http.StatusGatewayTimeout)
			log.Printf("TIMEOUT: Backend failed to respond in 2 seconds for %s\n", r.RemoteAddr)
			return
		}
		http.Error(w, "502 Bad Gateway - Backend is down", http.StatusBadGateway)
	}

	return proxy, nil
}

func gatewayMiddleware(limiter *limiter.IPRateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		if !limiter.Allow(host) {
			http.Error(w, "429 Too Many Requests - Rate limit exceeded", http.StatusTooManyRequests)
			log.Printf("RATE LIMIT: %s has exceeded the rate limit\n", host)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)

		defer cancel()

		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

func main() {
	godotenv.Load()
	gatewayPort := os.Getenv("GATEWAY_PORT")
	proxyPort := os.Getenv("PROXY_PORT")

	if gatewayPort == "" {
		gatewayPort = "9090"
	}
	if proxyPort == "" {
		proxyPort = "8080"
	}

	limiter := limiter.NewIPRateLimiter(5.0, 1.0)

	target := "http://localhost:" + gatewayPort
	proxyHandler, err := createProxyHandler(target)
	if err != nil {
		log.Fatalf("Failed to parse target URL: %v", err)
	}

	protectedHandler := gatewayMiddleware(limiter, proxyHandler)

	srv := &http.Server{
		Addr:    ":" + proxyPort,
		Handler: protectedHandler,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		fmt.Printf("Gateway is running on port %s\n", proxyPort)

		// ErrServerClosed is expected when we call srv.Shutdown(), so we don't log it as a crash
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Gateway server crashed: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("\nShutdown signal received. Commencing graceful shutdown of Gateway...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Graceful shutdown failed: %v\n", err)
	} else {
		log.Println("Gateway stopped accepting new requests and finished active ones.")
	}

	wg.Wait()
	log.Println("Gateway exited cleanly.")
}
