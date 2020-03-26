package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/hendrikmaus/openhab-auth-router/config"
	"github.com/hendrikmaus/openhab-auth-router/util"
	"github.com/sanity-io/litter"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

func main() {
	host := flag.String("host", "127.0.0.1", "Host to listen on")
	port := flag.String("port", "80", "Port to listen on")
	target := flag.String("target", "", "Address of your openHAB instance, e.g. 'http://openhab:8080'")
	configFilePath := flag.String("config", "", "Path to config.yaml")
	logLevel := flag.String("log-level", "info", "Loglevel as in [error|warn|info|debug]")
	flag.Parse()

	err := util.ConfigureLogger(logLevel)
	if err != nil {
		log.Fatal(err)
	}

	if len(*target) == 0 {
		log.Error("Please set '-target' to the address of your openHAB instance, e.g. 'http://openhab:8080'")
		os.Exit(1)
	}

	if len(*configFilePath) == 0 {
		log.Error("Please set '-config' to the path of your config.yaml file")
		os.Exit(1)
	}

	remote, err := url.Parse(*target)
	if err != nil {
		log.WithError(err).Errorf("Unable to parse target address '%s'", *target)
		os.Exit(1)
	}

	conf := config.Main{}
	data, err := ioutil.ReadFile(*configFilePath)
	if err != nil {
		log.WithError(err).Errorf("Could not read config file '%s'", *configFilePath)
		os.Exit(1)
	}

	err = yaml.Unmarshal(data, &conf)
	if err != nil {
		log.WithError(err).Error("Could not parse config file, please ensure it is valid YAML")
		os.Exit(1)
	}

	err = config.Validate(&conf)
	if err != nil {
		log.WithError(err).Error("Failed to validate config")
		os.Exit(1)
	}

	log.Debug(litter.Sdump(conf))

	proxy := httputil.NewSingleHostReverseProxy(remote)
	mux := http.NewServeMux()

	mux.HandleFunc("/liveness", livenessProbeHandler)
	mux.HandleFunc("/readiness", func(w http.ResponseWriter, r *http.Request) {
		readinessProbeHandler(w, r, remote)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		mainHandler(w, r, &conf, proxy)
	})

	addr := *host + ":" + *port
	log.Infof("Serving at %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.WithError(err).Error("Listener failed")
		os.Exit(1)
	}
}

func livenessProbeHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("liveness called")
	w.WriteHeader(http.StatusOK)
}

func readinessProbeHandler(w http.ResponseWriter, r *http.Request, remote *url.URL) {
	resp, err := http.Get(remote.String() + "/rest/")
	if err != nil || resp.StatusCode != http.StatusOK {
		log.Errorf("Readiness probe failed while trying to access '%s/rest/'", remote.String())
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	log.Debug("Readiness probe successful")
	w.WriteHeader(http.StatusOK)
}

func mainHandler(w http.ResponseWriter, r *http.Request, conf *config.Main, proxy *httputil.ReverseProxy) {
	user := r.Header.Get("X-Forwarded-Username")
	if len(user) == 0 && conf.Passthrough == false {
		failRequest(w, r, "The header 'X-Forwarded-Username' is either not set or empty")
		return
	}

	if conf.Passthrough == false {
		_, ok := conf.Users[user]
		if ok == false {
			log.Debugf("User '%s' not found in config; tried to access '%s'", user, r.URL.RequestURI())
			w.WriteHeader(403)
			return
		}

		log.Info(litter.Sdump(r.URL.RequestURI()))

		// Every user is forced to their entrypoint
		if r.URL.RequestURI() == "/" || r.URL.RequestURI() == "" {
			log.Debugf("Redirecting %s to users default entry-point %s", user, conf.Users[user].Entrypoint)
			http.Redirect(w, r, conf.Users[user].Entrypoint, http.StatusPermanentRedirect)
			return
		}

		// Check if the requested path is disallowed; if yes go to entrypoint
		for pathPart, pathConfig := range conf.Users[user].Paths {
			if strings.Contains(r.URL.RequestURI(), pathPart) {
				if pathConfig.Allowed == false {
					log.Debugf("Redirecting %s to users entrypoint %s - denying access to %s", user, conf.Users[user].Entrypoint, r.URL.RequestURI())
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
				log.Debugf("Redirecting %s to users default sitemap %s - no sitemap was given on the request", user, conf.Users[user].Sitemaps.Default)
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
				log.Debugf("Redirecting %s to users default sitemap %s - denying access to requested sitemap %s", user, conf.Users[user].Sitemaps.Default, sitemap)
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
				log.Debugf("Redirecting %s to users default sitemap %s - denying access to requested resource %s via REST API call", user, conf.Users[user].Sitemaps.Default, r.URL.RequestURI())
				http.Redirect(w, r, defaultSitemapURL, http.StatusPermanentRedirect)
				return
			}
		}
	}

serve:
	proxy.ServeHTTP(w, r)
}

func failRequest(w http.ResponseWriter, r *http.Request, message string) {
	if message != "" {
		log.Error(message)
	}
	contentType := r.Header.Get("Content-Type")

	switch {
	case strings.Contains(contentType, "text/html"):
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		if message != "" {
			_, err := fmt.Fprint(w, message)
			if err != nil {
				log.Fatal(err)
			}
		}
	case strings.Contains(contentType, "application/json"):
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		if message != "" {
			_, err := fmt.Fprintf(w, "{\"error\":\"%s\"", message)
			if err != nil {
				log.Fatal(err)
			}
		}
	default:
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		if message != "" {
			_, err := w.Write([]byte(message))
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}
