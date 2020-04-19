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

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}

func TestLivenessHandler(t *testing.T) {
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
	assert.Equal(t, "the header 'X-Forwarded-Username' is either not set or empty",
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

func Test_ruleDirector(t *testing.T) {
	type args struct {
		req  *http.Request
		conf *config.Main
	}
	tests := []struct {
		name string
		args args
		expectedRequestURI string
	}{
		{
			name: "user is forced to entrypoint",
			args: args{
				req: makeGETRequest("/", "test"),
				conf: &config.Main{
					Passthrough: false,
					Users:       map[string]*config.User{
						"test": {Entrypoint: "/start/index"},
					},
				},
			},
			expectedRequestURI: "/start/index",
		},
		{
			name: "user is forced to entrypoint with empty request uri",
			args: args{
				req: makeGETRequest("", "test"),
				conf: &config.Main{
					Passthrough: false,
					Users:       map[string]*config.User{
						"test": {Entrypoint: "/start/index"},
					},
				},
			},
			expectedRequestURI: "/start/index",
		},
		{
			name: "user is forced to entrypoint when trying to access disallowed path",
			args: args{
				req: makeGETRequest("/forbidden/path", "test"),
				conf: &config.Main{
					Passthrough: false,
					Users:       map[string]*config.User{
						"test": {
							Entrypoint: "/start/index",
							Paths:      map[string]*config.Path{
								"/forbidden/path": {
									Allowed: false,
								},
							},
						},
					},
				},
			},
			expectedRequestURI: "/start/index",
		},
		{
			name: "basicui - user is redirected to default sitemap when none is requested",
			args: args{
				req: makeGETRequest("/basicui/app", "test"),
				conf: &config.Main{
					Passthrough: false,
					Users:       map[string]*config.User{
						"test": {
							Entrypoint: "/start/index",
							Sitemaps: config.Sitemap{
								Default: "defaultSitemap",
								Allowed: nil,
							},
							Paths: map[string]*config.Path{
								"/forbidden/path": {
									Allowed: false,
								},
							},
						},
					},
				},
			},
			expectedRequestURI: "/basicui/app?sitemap=defaultSitemap",
		},
		{
			name: "basicui - user is redirected to default sitemap when a forbidden sitemap is requested",
			args: args{
				req: makeGETRequest("/basicui/app?sitemap=forbiddenSitemap", "test"),
				conf: &config.Main{
					Passthrough: false,
					Users:       map[string]*config.User{
						"test": {
							Entrypoint: "/start/index",
							Sitemaps: config.Sitemap{
								Default: "defaultSitemap",
								Allowed: nil,
							},
							Paths: map[string]*config.Path{
								"/forbidden/path": {
									Allowed: false,
								},
							},
						},
					},
				},
			},
			expectedRequestURI: "/basicui/app?sitemap=defaultSitemap",
		},
		{
			name: "basicui - user may access whitelisted sitemaps",
			args: args{
				req: makeGETRequest("/basicui/app?sitemap=allowedSitemap", "test"),
				conf: &config.Main{
					Passthrough: false,
					Users:       map[string]*config.User{
						"test": {
							Entrypoint: "/start/index",
							Sitemaps: config.Sitemap{
								Default: "defaultSitemap",
								Allowed: []string{"allowedSitemap"},
							},
							Paths: map[string]*config.Path{
								"/forbidden/path": {
									Allowed: false,
								},
							},
						},
					},
				},
			},
			expectedRequestURI: "/basicui/app?sitemap=allowedSitemap",
		},
		{
			name: "basicui - user may access all sitemaps when the wildcard is set",
			args: args{
				req: makeGETRequest("/basicui/app?sitemap=allowedSitemap", "test"),
				conf: &config.Main{
					Passthrough: false,
					Users:       map[string]*config.User{
						"test": {
							Entrypoint: "/start/index",
							Sitemaps: config.Sitemap{
								Default: "defaultSitemap",
								Allowed: []string{"*"},
							},
							Paths: map[string]*config.Path{
								"/forbidden/path": {
									Allowed: false,
								},
							},
						},
					},
				},
			},
			expectedRequestURI: "/basicui/app?sitemap=allowedSitemap",
		},
		{
			name: "rest - user may access events",
			args: args{
				req: makeGETRequest("/rest/sitemap/events", "test"),
				conf: &config.Main{
					Passthrough: false,
					Users:       map[string]*config.User{
						"test": {
							Entrypoint: "/start/index",
							Sitemaps: config.Sitemap{
								Default: "defaultSitemap",
								Allowed: []string{"*"},
							},
							Paths: map[string]*config.Path{
								"/forbidden/path": {
									Allowed: false,
								},
							},
						},
					},
				},
			},
			expectedRequestURI: "/rest/sitemap/events",
		},
		{
			name: "rest - user is redirected to default sitemap when it is requested explicitly",
			args: args{
				req: makeGETRequest("/rest/sitemaps/_default", "test"),
				conf: &config.Main{
					Passthrough: false,
					Users:       map[string]*config.User{
						"test": {
							Entrypoint: "/start/index",
							Sitemaps: config.Sitemap{
								Default: "defaultSitemap",
								Allowed: []string{"*"},
							},
							Paths: map[string]*config.Path{
								"/forbidden/path": {
									Allowed: false,
								},
							},
						},
					},
				},
			},
			expectedRequestURI: "/rest/sitemaps/defaultSitemap",
		},
		{
			name: "rest - user is redirected to default sitemap when requested sitemap is not allowed",
			args: args{
				req: makeGETRequest("/rest/sitemaps/forbiddenSitemap", "test"),
				conf: &config.Main{
					Passthrough: false,
					Users:       map[string]*config.User{
						"test": {
							Entrypoint: "/start/index",
							Sitemaps: config.Sitemap{
								Default: "defaultSitemap",
								Allowed: []string{},
							},
							Paths: map[string]*config.Path{
								"/forbidden/path": {
									Allowed: false,
								},
							},
						},
					},
				},
			},
			expectedRequestURI: "/rest/sitemaps/defaultSitemap",
		},
		{
			name: "rest - user may access allowed sitemaps",
			args: args{
				req: makeGETRequest("/rest/sitemaps/allowedSitemap", "test"),
				conf: &config.Main{
					Passthrough: false,
					Users:       map[string]*config.User{
						"test": {
							Entrypoint: "/start/index",
							Sitemaps: config.Sitemap{
								Default: "defaultSitemap",
								Allowed: []string{"allowedSitemap"},
							},
							Paths: map[string]*config.Path{
								"/forbidden/path": {
									Allowed: false,
								},
							},
						},
					},
				},
			},
			expectedRequestURI: "/rest/sitemaps/allowedSitemap",
		},
		{
			name: "rest - user may access all sitemaps when a wildcard is set",
			args: args{
				req: makeGETRequest("/rest/sitemaps/allowedSitemap", "test"),
				conf: &config.Main{
					Passthrough: false,
					Users:       map[string]*config.User{
						"test": {
							Entrypoint: "/start/index",
							Sitemaps: config.Sitemap{
								Default: "defaultSitemap",
								Allowed: []string{"*"},
							},
							Paths: map[string]*config.Path{
								"/forbidden/path": {
									Allowed: false,
								},
							},
						},
					},
				},
			},
			expectedRequestURI: "/rest/sitemaps/allowedSitemap",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ruleDirector(tt.args.req, tt.args.conf)
			assert.Equal(t, tt.expectedRequestURI, tt.args.req.URL.RequestURI())
		})
	}
}

func makeGETRequest(uri string, user string) *http.Request {
	r, _ := http.NewRequest("GET", uri, nil)
	r.Header.Add("X-Forwarded-Username", user)
	return r
}
