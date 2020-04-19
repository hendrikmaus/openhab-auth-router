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
	Remote *url.URL
	Config *config.Main
}

func (o *Options) Validate() error {
	if o.Target == "" {
		return errors.New("please set '-target' to the address of your openHAB instance, e.g. 'http://openhab:8080'")
	}

	if o.ConfigFilePath == "" {
		return errors.New("please set '-config' to the path of your config.yaml file")
	}

	remote, err := url.Parse(o.Target)
	if err != nil {
		return errors.New(fmt.Sprintf("unable to parse target address '%s'", o.Target))
	}
	o.Remote = remote

	o.Config = &config.Main{}
	data, err := ioutil.ReadFile(o.ConfigFilePath)
	if err != nil{
		return errors.New(fmt.Sprintf("could not read config file '%s'", o.ConfigFilePath))
	}

	err = yaml.Unmarshal(data, o.Config)
	if err != nil {
		return errors.New("could not parse config file, please ensure it is valid YAML")
	}

	err = config.Validate(o.Config)
	if err != nil {
		return errors.New("failed to validate config")
	}
	
	return nil
}

type Router struct {
	Log  zerolog.Logger
	Opts *Options
}

func main() {
	opts := Options{}
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
	
	log.Debug().Interface("config", opts).Msg("processed concfiguration")

	router := Router{
		Log:  logger,
		Opts: &opts,
	}

	proxy := httputil.NewSingleHostReverseProxy(opts.Remote)
	mux := http.NewServeMux()

	mux.HandleFunc("/liveness", router.LivenessProbeHandler)
	mux.HandleFunc("/readiness", func(w http.ResponseWriter, r *http.Request) {
		router.ReadinessProbeHandler(w, r, router.Opts.Remote)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		mainHandler(w, r, router.Opts.Config, proxy)
	})

	log.Info().Str("host", router.Opts.Host).Str("port", router.Opts.Port).Msg("serving")
	if err := http.ListenAndServe(fmt.Sprintf("%s:%s", router.Opts.Host, router.Opts.Port), mux); err != nil {
		log.Fatal().Err(err).Msg("failed serving")
	}
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

func mainHandler(w http.ResponseWriter, r *http.Request, conf *config.Main, proxy *httputil.ReverseProxy) {
	user := r.Header.Get("X-Forwarded-Username")
	if user == "" && conf.Passthrough == false {
		failRequest(w, r, "The header 'X-Forwarded-Username' is either not set or empty")
		return
	}

	logger := log.With().Str("user", user).Logger()

	if conf.Passthrough == false {
		_, ok := conf.Users[user]
		if ok == false {
			logger.Debug().Msgf("User '%s' not found in config; tried to access '%s'", user, r.URL.RequestURI())
			w.WriteHeader(403)
			return
		}

		logger.Debug().Str("uri", r.URL.RequestURI()).Msg("")

		// Every user is forced to their entrypoint
		if r.URL.RequestURI() == "/" || r.URL.RequestURI() == "" {
			logger.Debug().Msgf("redirecting to default entry-point %s", conf.Users[user].Entrypoint)
			http.Redirect(w, r, conf.Users[user].Entrypoint, http.StatusPermanentRedirect)
			return
		}

		// Check if the requested path is disallowed; if yes go to entrypoint
		for pathPart, pathConfig := range conf.Users[user].Paths {
			if strings.Contains(r.URL.RequestURI(), pathPart) {
				if pathConfig.Allowed == false {
					logger.Debug().Msgf("redirecting to default entrypoint %s - denying access to %s", conf.Users[user].Entrypoint, r.URL.RequestURI())
					http.Redirect(w, r, conf.Users[user].Entrypoint, http.StatusPermanentRedirect)
					return
				}
			}
		}

		// Handle basicui access
		if strings.HasPrefix(r.URL.RequestURI(), "/basicui/app") {
			queryString := r.URL.Query()
			sitemap := queryString.Get("sitemap")
			if sitemap == "" {
				queryString.Set("sitemap", conf.Users[user].Sitemaps.Default)
				r.URL.RawQuery = queryString.Encode()
				logger.Debug().Msgf("redirecting to default sitemap %s - no sitemap was given on the request", conf.Users[user].Sitemaps.Default)
				http.Redirect(w, r, r.URL.String(), http.StatusPermanentRedirect)
				return
			}
			if sitemap != "" && sitemap != conf.Users[user].Sitemaps.Default {
				if len(conf.Users[user].Sitemaps.Allowed) == 1 && conf.Users[user].Sitemaps.Allowed[0] == "*" {
					goto serve
				}
				for _, allowedSitemap := range conf.Users[user].Sitemaps.Allowed {
					if sitemap == allowedSitemap {
						goto serve
					}
				}
				queryString.Set("sitemap", conf.Users[user].Sitemaps.Default)
				r.URL.RawQuery = queryString.Encode()
				logger.Debug().Msgf("redirecting to default sitemap %s - denying access to requested sitemap %s", conf.Users[user].Sitemaps.Default, sitemap)
				http.Redirect(w, r, r.URL.String(), http.StatusPermanentRedirect)
				return
			}
		}

		// Handle rest access
		if strings.HasPrefix(r.URL.RequestURI(), "/rest") {
			if strings.HasPrefix(r.URL.RequestURI(), "/rest/sitemaps/events") {
				goto serve
			}
			if strings.HasPrefix(r.URL.RequestURI(), "/rest/sitemaps/_default") {
				http.Redirect(w, r, "/rest/sitemaps/"+conf.Users[user].Sitemaps.Default, http.StatusPermanentRedirect)
				return
			}
			if strings.HasPrefix(r.URL.RequestURI(), "/rest/sitemaps/") {
				if len(conf.Users[user].Sitemaps.Allowed) == 1 && conf.Users[user].Sitemaps.Allowed[0] == "*" {
					goto serve
				}
				for _, allowedSitemap := range conf.Users[user].Sitemaps.Allowed {
					if strings.Contains(r.URL.RequestURI(), allowedSitemap) {
						goto serve
					}
				}
				parts := strings.Split(r.URL.RequestURI(), "/")
				defaultSitemapURL := strings.Replace(r.URL.RequestURI(), parts[3], conf.Users[user].Sitemaps.Default, -1)
				logger.Debug().Msgf("redirecting to default sitemap %s - denying access to requested resource %s via REST API call", conf.Users[user].Sitemaps.Default, r.URL.RequestURI())
				http.Redirect(w, r, defaultSitemapURL, http.StatusPermanentRedirect)
				return
			}
		}
	} else {
		logger.Debug().Str("uri", r.URL.RequestURI()).Msg("passthrough request served")
	}

serve:
	proxy.ServeHTTP(w, r)
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
