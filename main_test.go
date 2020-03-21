package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestLivenessHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/liveness", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(livenessProbeHandler)
	handler.ServeHTTP(rr, req)

	status := rr.Code
	if status != http.StatusOK {
		t.Errorf("handler responded with wrong status code: got %v wanted %v",
			status, http.StatusOK)
	}
}

func TestReadinessHandlerFailsConnectingToRemote(t *testing.T) {
	req, err := http.NewRequest("GET", "/readiness", nil)
	if err != nil {
		t.Fatal(err)
	}

	remoteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer remoteServer.Close()
	remote, _ := url.Parse(remoteServer.URL)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		readinessProbeHandler(w, r, remote)
	})
	handler.ServeHTTP(rr, req)

	status := rr.Code
	if status != http.StatusServiceUnavailable {
		t.Errorf("handler responded with wrong status code: got %v wanted %v",
			status, http.StatusServiceUnavailable)
	}
}

func TestReadinessHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/readiness", nil)
	if err != nil {
		t.Fatal(err)
	}

	remoteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer remoteServer.Close()
	remote, _ := url.Parse(remoteServer.URL)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		readinessProbeHandler(w, r, remote)
	})
	handler.ServeHTTP(rr, req)

	status := rr.Code
	if status != http.StatusOK {
		t.Errorf("handler responded with wrong status code: got %v wanted %v",
			status, http.StatusOK)
	}
}
