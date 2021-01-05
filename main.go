package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/hendrikmaus/openhab-auth-router/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
)

type Options struct {
	Host           string
	Port           string
	Target         string
	ConfigFilePath string
	LogLevel       string
}

func (o *Options) Validate() error {
	if o.Target == "" {
		return errors.New("please set '-target' to the address of your openHAB instance, e.g. 'http://openhab:8080'")
	}

	if o.ConfigFilePath == "" {
		return errors.New("please set '-config' to the path of your config.yaml file")
	}

	return nil
}

type Router struct {
	Log  zerolog.Logger
	Opts *Options
	Config *config.Main
}

func main() {
	opts := &Options{}
	flag.StringVar(&opts.Host, "host", "127.0.0.1", "Host to listen on")
	flag.StringVar(&opts.Port, "port", "80", "Port to listen on")
	flag.StringVar(&opts.Target, "target", "", "Address of your openHAB instance, e.g. 'http://openhab:8080'")
	flag.StringVar(&opts.ConfigFilePath, "config", "", "Path to config.yaml")
	flag.StringVar(&opts.LogLevel, "log-level", "info", "Loglevel as in [error|warn|info|debug]")
	flag.Parse()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

	switch opts.LogLevel {
	case "error":
		logger = logger.Level(zerolog.ErrorLevel)
	case "warn":
		logger = logger.Level(zerolog.WarnLevel)
	case "info":
		logger = logger.Level(zerolog.InfoLevel)
	case "debug":
		logger = logger.Level(zerolog.DebugLevel)
	default:
		logger = logger.Level(zerolog.InfoLevel)
	}

	if err := opts.Validate(); err != nil {
		log.Fatal().Err(err).Msg("invalid options, exiting")
	}

	conf := &config.Main{}
	data, err := ioutil.ReadFile(opts.ConfigFilePath)
	if err != nil {
		log.Fatal().Msgf("could not read config file '%s'", opts.ConfigFilePath)
	}

	if err := yaml.Unmarshal(data, conf); err != nil {
		log.Fatal().Msg("could not parse config file, please ensure it is valid YAML")
	}

	if err := config.Validate(conf); err != nil {
		log.Fatal().Msg("failed to validate config")
	}

	log.Debug().Interface("config", opts).Msg("processed configuration")

	router := &Router{
		Log:  logger,
		Opts: opts,
		Config: conf,
	}

	proxy := router.MakeProxy()
	mux := router.MakeMux(proxy)

	log.Info().Str("host", router.Opts.Host).Str("port", router.Opts.Port).Msg("serving")
	if err := http.ListenAndServe(fmt.Sprintf("%s:%s", router.Opts.Host, router.Opts.Port), mux); err != nil {
		log.Fatal().Err(err).Msg("failed serving")
	}
}

func (r *Router) MakeProxy() *httputil.ReverseProxy {
	remote, err := url.Parse(r.Opts.Target)
	if err != nil {
		log.Fatal().Msg("failed to parse given target")
	}
	proxy := httputil.NewSingleHostReverseProxy(remote)
	defaultDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		defaultDirector(req)
		ruleDirector(req, r.Config)
	}
	return proxy
}

func (r *Router) MakeMux(proxy *httputil.ReverseProxy) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/liveness", r.LivenessProbeHandler)

	remote, err := url.Parse(r.Opts.Target)
	if err != nil {
		log.Fatal().Msg("failed to parse given target")
	}
	mux.HandleFunc("/readiness", func(w http.ResponseWriter, req *http.Request) {
		r.ReadinessProbeHandler(w, req, remote)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		mainHandler(w, req, r.Config, proxy)
	})

	return mux
}

// LivenessProbeHandler responds with HTTP 200
func (r *Router) LivenessProbeHandler(w http.ResponseWriter, req *http.Request) {
	r.Log.Debug().Str("probe", "liveness").Msg("")
	w.WriteHeader(http.StatusOK)
}

// ReadinessProbeHandler asserts connection to downstream dependencies
func (r *Router) ReadinessProbeHandler(w http.ResponseWriter, req *http.Request, remote *url.URL) {
	resp, err := http.Get(remote.String() + "/rest/")
	if err != nil || resp.StatusCode != http.StatusOK {
		r.Log.Err(err).Str("probe", "readiness").Str("remote", remote.String()).Msg("failed to assert target access")
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	r.Log.Debug().Str("probe", "readiness").Msg("")
	w.WriteHeader(http.StatusOK)
}

func ruleDirector(req *http.Request, conf *config.Main) {
	user := req.Header.Get("X-Forwarded-Username")
	logger := log.With().Str("user", user).Logger()

	if conf.Passthrough {
		logger.Debug().Str("uri", req.URL.RequestURI()).Msg("passthrough request served")
		return
	}

	// Every user is forced to their entrypoint
	if req.URL.RequestURI() == "/" || req.URL.RequestURI() == "" {
		logger.Debug().Msgf("redirecting to default entry-point %s", conf.Users[user].Entrypoint)
		req.URL.Path = conf.Users[user].Entrypoint
	}

	// Check if the requested path is disallowed; if yes go to entrypoint
	for pathPart, pathConfig := range conf.Users[user].Paths {
		if strings.Contains(req.URL.RequestURI(), pathPart) {
			if pathConfig.Allowed == false {
				logger.Debug().Msgf("redirecting to default entrypoint %s - denying access to %s", conf.Users[user].Entrypoint, req.URL.RequestURI())
				req.URL.Path = conf.Users[user].Entrypoint
			}
		}
	}

	// Handle basicui access
	if strings.HasPrefix(req.URL.RequestURI(), "/basicui/app") {
		queryString := req.URL.Query()
		sitemap := queryString.Get("sitemap")
		if sitemap == "" {
			queryString.Set("sitemap", conf.Users[user].Sitemaps.Default)
			req.URL.RawQuery = queryString.Encode()
			logger.Debug().Msgf("redirecting to default sitemap %s - no sitemap was given on the request", conf.Users[user].Sitemaps.Default)
			return
		}
		if sitemap != "" && sitemap != conf.Users[user].Sitemaps.Default {
			if len(conf.Users[user].Sitemaps.Allowed) == 1 && conf.Users[user].Sitemaps.Allowed[0] == "*" {
				return
			}
			for _, allowedSitemap := range conf.Users[user].Sitemaps.Allowed {
				if sitemap == allowedSitemap {
					return
				}
			}
			queryString.Set("sitemap", conf.Users[user].Sitemaps.Default)
			req.URL.RawQuery = queryString.Encode()
			logger.Debug().Msgf("redirecting to default sitemap %s - denying access to requested sitemap %s", conf.Users[user].Sitemaps.Default, sitemap)
			return
		}
	}

	// Handle rest access
	if strings.HasPrefix(req.URL.RequestURI(), "/rest") {
		if strings.HasPrefix(req.URL.RequestURI(), "/rest/sitemaps/events") {
			return
		}
		if strings.HasPrefix(req.URL.RequestURI(), "/rest/sitemaps/_default") {
			req.URL.Path = "/rest/sitemaps/"+conf.Users[user].Sitemaps.Default
			return
		}
		if strings.HasPrefix(req.URL.RequestURI(), "/rest/sitemaps/") {
			if len(conf.Users[user].Sitemaps.Allowed) == 1 && conf.Users[user].Sitemaps.Allowed[0] == "*" {
				return
			}
			for _, allowedSitemap := range conf.Users[user].Sitemaps.Allowed {
				if strings.Contains(req.URL.RequestURI(), allowedSitemap) {
					return
				}
			}
			parts := strings.Split(req.URL.RequestURI(), "/")
			defaultSitemapURL := strings.Replace(req.URL.RequestURI(), parts[3], conf.Users[user].Sitemaps.Default, -1)
			logger.Debug().Msgf("redirecting to default sitemap %s - denying access to requested resource %s via REST API call", conf.Users[user].Sitemaps.Default, req.URL.RequestURI())
			req.URL.Path = defaultSitemapURL
			return
		}
	}
}

func mainHandler(w http.ResponseWriter, req *http.Request, conf *config.Main, proxy *httputil.ReverseProxy) {
	if conf.Passthrough == false {
		user := req.Header.Get("X-Forwarded-Username")
		if user == "" && conf.Passthrough == false {
			failRequest(w, req, "the header 'X-Forwarded-Username' is either not set or empty")
			return
		}

		_, ok := conf.Users[user]
		if ok == false {
			log.Debug().Str("user", user).Str("uri", req.URL.RequestURI()).Msg("user not found")
			w.WriteHeader(403)
			return
		}
	}

	proxy.ServeHTTP(w, req)
}

func failRequest(w http.ResponseWriter, r *http.Request, message string) {
	if message != "" {
		log.Error().Msg(message)
	}
	contentType := r.Header.Get("Content-Type")

	switch {
	case strings.Contains(contentType, "text/html"):
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		if message != "" {
			_, err := fmt.Fprint(w, message)
			if err != nil {
				log.Fatal().Err(err).Msg("")
			}
		}
	case strings.Contains(contentType, "application/json"):
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		if message != "" {
			_, err := fmt.Fprintf(w, "{\"error\":\"%s\"", message)
			if err != nil {
				log.Fatal().Err(err).Msg("")
			}
		}
	default:
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		if message != "" {
			_, err := w.Write([]byte(message))
			if err != nil {
				log.Fatal().Err(err).Msg("")
			}
		}
	}
}
