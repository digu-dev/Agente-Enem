package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()

	newHandler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got, want := response.Body.String(), "{\"status\":\"ok\"}\n"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestHealthHandlerRejectsNonGet(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/health", nil)
	response := httptest.NewRecorder()

	newHandler().ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
	if got, want := response.Header().Get("Allow"), http.MethodGet; got != want {
		t.Fatalf("Allow = %q, want %q", got, want)
	}
}

func TestPortAddress(t *testing.T) {
	t.Setenv("PORT", "9090")

	got, err := portAddress()
	if err != nil {
		t.Fatalf("portAddress() error = %v", err)
	}
	if want := ":9090"; got != want {
		t.Fatalf("portAddress() = %q, want %q", got, want)
	}
}

func TestPortAddressRejectsInvalidPort(t *testing.T) {
	t.Setenv("PORT", "70000")

	if _, err := portAddress(); err == nil {
		t.Fatal("portAddress() error = nil, want invalid PORT error")
	}
}
