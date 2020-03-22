package main

import (
	"github.com/hendrikmaus/openhab-auth-router/config"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
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
		t.Errorf("handler responded with wrong status code: got '%v' wanted '%v'",
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
		t.Errorf("handler responded with wrong status code: got '%v' wanted '%v'",
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
		t.Errorf("handler responded with wrong status code: got '%v' wanted '%v'",
			status, http.StatusOK)
	}
}

func TestMissingHeaderInNonPassthroughModeFails(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	conf := config.Main{Passthrough:false}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mainHandler(w, r, &conf, nil)
	})
	handler.ServeHTTP(rr, req)

	status := rr.Code
	if status != http.StatusBadRequest {
		t.Errorf("handler responded with wrong status code: got %v wanted %v", status, http.StatusBadRequest)
	}

	expected := "The header 'X-Forwarded-Username' is either not set or empty"
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got '%v' wanted '%v'", rr.Body.String(), expected)
	}
}

func TestPassthrough(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	remoteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer remoteServer.Close()
	remote, _ := url.Parse(remoteServer.URL)
	proxy := httputil.NewSingleHostReverseProxy(remote)

	conf := config.Main{Passthrough:true}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mainHandler(w, r, &conf, proxy)
	})
	handler.ServeHTTP(rr, req)

	status := rr.Code
	if status != http.StatusOK {
		t.Errorf("handler responded with wrong status code: got %v wanted %v", status, http.StatusOK)
	}
}

func TestUnknownUserIsBlocked(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("X-Forwarded-Username", "test")

	conf := config.Main{Passthrough:false}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mainHandler(w, r, &conf, nil)
	})
	handler.ServeHTTP(rr, req)

	status := rr.Code
	if status != http.StatusForbidden {
		t.Errorf("handler responded with wrong status code: got %v wanted %v", status, http.StatusForbidden)
	}
}

func TestUserIsRedirectedToDefaultEntrypoint(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("X-Forwarded-Username", "test")

	user := config.User{Entrypoint:"/start/index"}
	conf := config.Main{
		Passthrough: false,
		Users: map[string]*config.User{
			"test": &user,
		},
	}

	remoteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/start/index", r.RequestURI)
		w.WriteHeader(http.StatusOK)
	}))
	defer remoteServer.Close()
	remote, _ := url.Parse(remoteServer.URL)
	proxy := httputil.NewSingleHostReverseProxy(remote)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mainHandler(w, r, &conf, proxy)
	})
	handler.ServeHTTP(rr, req)
}
