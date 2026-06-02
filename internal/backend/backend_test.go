package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHelloHandler(test *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		test.Fatalf("failed to create GET request: %v", err)
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(HelloHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		test.Errorf("handler returned wrong status code: Got: %v Expected: %v",
			status, http.StatusOK)
	}

	expected := "Hello from Backend\n"
	if rr.Body.String() != expected {
		test.Errorf("handler returned unexpected body: Got: %v Expected: %v",
			rr.Body.String(), expected)
	}
}
