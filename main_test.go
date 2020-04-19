package main

import (
	"github.com/hendrikmaus/openhab-auth-router/config"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"
)

func TestLivenessHandler(t *testing.T) {
	// silence logs for all test cases
	zerolog.SetGlobalLevel(zerolog.Disabled)

	// ---

	req, err := http.NewRequest("GET", "/liveness", nil)
	if err != nil {
		t.Fatal(err)
	}

	router := Router{}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(router.LivenessProbeHandler)
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

	router := Router{}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		router.ReadinessProbeHandler(w, r, remote)
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

	router := Router{}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		router.ReadinessProbeHandler(w, r, remote)
	})
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestMissingHeaderInNonPassthroughModeFails(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	conf := config.Main{Passthrough: false}
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

	conf := config.Main{Passthrough: true}
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

	conf := config.Main{Passthrough: false}
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

	user := config.User{Entrypoint: "/start/index"}
	conf := config.Main{
		Passthrough: false,
		Users: map[string]*config.User{
			"test": &user,
		},
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mainHandler(w, r, &conf, nil)
	})
	handler.ServeHTTP(rr, req)

	loc, _ := rr.Result().Location()
	assert.Equal(t, "/start/index", loc.RequestURI())
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
				Entrypoint: "/default/entrypoint",
				Paths: map[string]*config.Path{
					"/start/index": {Allowed: false},
				},
			},
		},
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mainHandler(w, r, &conf, nil)
	})
	handler.ServeHTTP(rr, req)

	loc, _ := rr.Result().Location()
	assert.Equal(t, "/default/entrypoint", loc.RequestURI())
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

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mainHandler(w, r, &conf, nil)
	})
	handler.ServeHTTP(rr, req)

	loc, _ := rr.Result().Location()
	assert.Equal(t, "/basicui/app?sitemap=test_sitemap", loc.RequestURI())
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

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mainHandler(w, r, &conf, nil)
	})
	handler.ServeHTTP(rr, req)

	loc, _ := rr.Result().Location()
	assert.Equal(t, "/basicui/app?sitemap=test_sitemap", loc.RequestURI())
}

func TestSitemapAccessWildcard(t *testing.T) {
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
					Allowed: []string{"*"},
				},
			},
		},
	}

	remoteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/basicui/app?sitemap=admin", r.URL.RequestURI())
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

func TestSitemapAccessToAllowedSitemaps(t *testing.T) {
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
					Allowed: []string{"admin", "default"},
				},
			},
		},
	}

	remoteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/basicui/app?sitemap=admin", r.URL.RequestURI())
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

func TestRestEvents(t *testing.T) {
	req, err := http.NewRequest("GET", "/rest/sitemaps/events", nil)
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
					Allowed: []string{"admin", "default"},
				},
			},
		},
	}

	remoteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rest/sitemaps/events", r.URL.RequestURI())
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

func TestRestDefaultSitemapRedirect(t *testing.T) {
	req, err := http.NewRequest("GET", "/rest/sitemaps/_default", nil)
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
					Allowed: []string{"admin", "default"},
				},
			},
		},
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mainHandler(w, r, &conf, nil)
	})
	handler.ServeHTTP(rr, req)

	loc, _ := rr.Result().Location()
	assert.Equal(t, "/rest/sitemaps/test_sitemap", loc.RequestURI())
}

func TestRestUserIsRedirectedToDefaultSitemapWhenDisallowed(t *testing.T) {
	req, err := http.NewRequest("GET", "/rest/sitemaps/admin", nil)
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
					Allowed: []string{"default"},
				},
			},
		},
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mainHandler(w, r, &conf, nil)
	})
	handler.ServeHTTP(rr, req)

	loc, _ := rr.Result().Location()
	assert.Equal(t, "/rest/sitemaps/test_sitemap", loc.RequestURI())
}

func TestRestSitemapAccessWildcard(t *testing.T) {
	req, err := http.NewRequest("GET", "/rest/sitemaps/admin", nil)
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
					Allowed: []string{"*"},
				},
			},
		},
	}

	remoteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rest/sitemaps/admin?sitemap=admin", r.URL.RequestURI())
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

func TestRestSitemapAccessToAllowedSitemaps(t *testing.T) {
	req, err := http.NewRequest("GET", "/rest/sitemaps/admin", nil)
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
					Allowed: []string{"admin", "default"},
				},
			},
		},
	}

	remoteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rest/sitemaps/admin?sitemap=admin", r.URL.RequestURI())
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
