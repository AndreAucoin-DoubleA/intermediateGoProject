package main

import (
	"context"
	"errors"
	"intermediate/internal/limiter"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReverseProxy(test *testing.T) {
	mockGateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello from Mock Gateway"))
	}))
	defer mockGateway.Close()

	proxyHandler, err := createProxyHandler(mockGateway.URL)
	if err != nil {
		test.Fatalf("Failed to create proxy handler: %v", err)
	}

	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		test.Fatalf("Failed to create GET request: %v", err)
	}
	rr := httptest.NewRecorder()
	proxyHandler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		test.Errorf("Proxy handler returned wrong status code: Got: %v Expected: %v",
			status, http.StatusOK)
	}

	body, err := io.ReadAll(rr.Body)
	if err != nil {
		test.Fatalf("Failed to read response body: %v", err)
	}

	expected := "Hello from Mock Gateway"
	if string(body) != expected {
		test.Errorf("Proxy handler returned unexpected body: Got: %v Expected: %v",
			string(body), expected)
	}

}

func TestGatewayMiddleware_RateLimiting(test *testing.T) {
	limiter := limiter.NewIPRateLimiter(1.0, 0.0)

	mockBackend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := gatewayMiddleware(limiter, mockBackend)

	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.RemoteAddr = "192.168.1.50:12345"
	rr1 := httptest.NewRecorder()

	handler.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		test.Errorf("Expected first request to return 200 OK, got %d", rr1.Code)
	}

	// --- Request 2 (Should Fail with 429) ---
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "192.168.1.50:54321" // Same IP, different port (like a real request)
	rr2 := httptest.NewRecorder()

	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusTooManyRequests {
		test.Errorf("Expected second request to return 429 Too Many Requests, got %d", rr2.Code)
	}
}

func TestGatewayMiddleware_ContextTimeout(test *testing.T) {
	limiter := limiter.NewIPRateLimiter(10.0, 1.0)

	mockBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))

	defer mockBackend.Close()

	proxy, err := createProxyHandler(mockBackend.URL)
	if err != nil {
		test.Fatalf("Failed to create proxy: %v", err)
	}

	handler := gatewayMiddleware(limiter, proxy)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:9999"
	rr := httptest.NewRecorder()

	start := time.Now()

	handler.ServeHTTP(rr, req)

	duration := time.Since(start)

	// Assert 1: The response code must be 504 Gateway Timeout
	if rr.Code != http.StatusGatewayTimeout {
		test.Errorf("Expected 504 Gateway Timeout, got %d", rr.Code)
	}

	// Assert 2: The duration should be around 2 seconds.
	// If it takes 3 seconds, the context timeout failed to cancel the proxy!
	if duration >= 3*time.Second {
		test.Errorf("Test took %v. The context timeout failed to sever the connection early!", duration)
	}
}

func TestGracefulShutdown_WaitsForActiveRequests(test *testing.T) {
	slowHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Second)
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{
		Addr:    "127.0.0.1:0",
		Handler: slowHandler,
	}

	listener, err := net.Listen("tcp", srv.Addr)

	if err != nil {
		test.Fatalf("Failed to create listener: %v", err)
	}

	serverURL := "http://" + listener.Addr().String()

	go func() {
		if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			test.Errorf("Server crashed unexpectedly: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	requestCompleted := make(chan bool)

	go func() {
		resp, err := http.Get(serverURL)
		if err != nil {
			test.Errorf("Request failed during shutdown: %v", err)
			requestCompleted <- false
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			test.Errorf("Expected 200 OK, got %d", resp.StatusCode)
		}
		requestCompleted <- true
	}()

	time.Sleep(100 * time.Millisecond)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	shutdownStart := time.Now()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		test.Fatalf("Graceful shutdown failed: %v", err)
	}

	shutdownDuration := time.Since(shutdownStart)

	success := <-requestCompleted
	if !success {
		test.Fatal("The active request was abruptly killed instead of finishing!")
	}

	if shutdownDuration < 800*time.Millisecond {
		test.Errorf("Shutdown completed too quickly (%v). It did not wait for the active request!", shutdownDuration)
	}
}
