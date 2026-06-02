package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/joho/godotenv"
)

func createProxyHandler(target string) (http.Handler, error) {
	targetURL, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	return httputil.NewSingleHostReverseProxy(targetURL), nil
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

	target := "http://localhost:" + gatewayPort
	proxyHandler, err := createProxyHandler(target)
	if err != nil {
		log.Fatalf("Failed to parse target URL: %v", err)
	}

	http.Handle("/", proxyHandler)

	fmt.Printf("Gateway is running on port %s\n", proxyPort)
	if err := http.ListenAndServe(":"+proxyPort, nil); err != nil {
		log.Fatal(err)
	}
}
