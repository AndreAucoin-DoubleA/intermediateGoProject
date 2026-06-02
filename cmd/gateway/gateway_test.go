package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
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
