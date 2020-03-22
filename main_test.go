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

	assert.Equal(t, http.StatusOK, rr.Code)
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

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
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

	assert.Equal(t, http.StatusOK, rr.Code)
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

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, "The header 'X-Forwarded-Username' is either not set or empty",
		rr.Body.String())
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

	assert.Equal(t, http.StatusOK, rr.Code)
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

	assert.Equal(t, http.StatusForbidden, rr.Code)
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
		assert.Equal(t, "/start/index", r.URL.RequestURI())
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

func TestUserIsRedirectedToDefaultEntrypointOnDisallowedPath(t *testing.T) {
	req, err := http.NewRequest("GET", "/start/index", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("X-Forwarded-Username", "test")

	conf := config.Main{
		Passthrough: false,
		Users: map[string]*config.User{
			"test": {
				Entrypoint:"/default/entrypoint",
				Paths: map[string]*config.Path{
					"/start/index": {Allowed:false},
				},
			},
		},
	}

	remoteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/default/entrypoint", r.URL.RequestURI())
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

func TestUserIsRedirectedToDefaultSitemapWhenNoSitemapIsGiven(t *testing.T) {
	req, err := http.NewRequest("GET", "/basicui/app", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("X-Forwarded-Username", "test")

	conf := config.Main{
		Passthrough: false,
		Users: map[string]*config.User{
			"test": {
				Sitemaps: config.Sitemap{
					Default: "test_sitemap",
				},
			},
		},
	}

	remoteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/basicui/app", r.URL.RequestURI())
		assert.Equal(t, conf.Users["test"].Sitemaps.Default, r.URL.Query().Get("sitemap"))
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

func TestUserIsRedirectedToDefaultSitemapWhenAccessIsDenied(t *testing.T) {
	req, err := http.NewRequest("GET", "/basicui/app", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("X-Forwarded-Username", "test")
	q := req.URL.Query()
	q.Set("sitemap", "admin")
	req.URL.RawQuery = q.Encode()

	conf := config.Main{
		Passthrough: false,
		Users: map[string]*config.User{
			"test": {
				Sitemaps: config.Sitemap{
					Default: "test_sitemap",
				},
			},
		},
	}

	remoteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/basicui/app", r.URL.RequestURI())
		assert.Equal(t, conf.Users["test"].Sitemaps.Default, r.URL.Query().Get("sitemap"))
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
