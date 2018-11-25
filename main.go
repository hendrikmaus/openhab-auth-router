package main

import (
	"flag"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/hendrikmaus/openhab-auth-router/config"
	"github.com/hendrikmaus/openhab-auth-router/util"
	"github.com/sanity-io/litter"
	"github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
)

func main() {
	host := flag.String("host", "127.0.0.1", "Host to listen on")
	port := flag.String("port", "80", "Port to listen on")
	target := flag.String("target", "", "Address of your OpenHAB instance, e.g. 'http://openhab:8080'")
	configFilePath := flag.String("config", "", "Path to config.yaml")
	logLevel := flag.String("log-level", "info", "Loglevel as in [error|warn|info|debug]")
	logType := flag.String("log-type", "auto", "Set the type of logging [human|human-color|machine|machine+color|auto]")
	flag.Parse()

	util.ConfigureLogger(logLevel, logType)

	if len(*target) == 0 {
		logrus.Error("Please set '-target' to the address of your OpenHAB instance, e.g. 'http://openhab:8080'")
		os.Exit(1)
	}

	if len(*configFilePath) == 0 {
		logrus.Error("Please set '-config' to the path of your config.yaml file")
		os.Exit(1)
	}

	remote, err := url.Parse(*target)
	if err != nil {
		logrus.WithError(err).Errorf("Unable to parse target address '%s'", *target)
		os.Exit(1)
	}

	config := config.Main{}
	data, err := ioutil.ReadFile(*configFilePath)
	if err != nil {
		logrus.WithError(err).Errorf("Could not read config file '%s'", *configFilePath)
		os.Exit(1)
	}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		logrus.WithError(err).Error("Could not parse config file, please ensure it is valid YAML")
		os.Exit(1)
	}

	logrus.Debug(litter.Sdump(config))

	proxy := httputil.NewSingleHostReverseProxy(remote)
	mux := http.NewServeMux()
	mux.HandleFunc("/liveness", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/readiness", func(w http.ResponseWriter, r *http.Request) {
		resp, err := http.Get(remote.String() + "/rest/")
		if err != nil || resp.StatusCode != http.StatusOK {
			logrus.Errorf("Readiness probe failed while trying to access '%s/rest/'", remote.String())
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		logrus.Debug("Readiness probe successful")
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		user := r.Header.Get("X-Forwarded-Username")
		// TODO: how to behave if there is no user? deny all if passthrough is set to true
		if len(user) != 0 {
			logrus.Debugf("Detected user '%s'", user)
		} else {
			logrus.Debug("No user detected, either 'X-Forwarded-Username' header is not set or empty")
		}

		if config.Passthrough == false {
			_, ok := config.Users[user]
			if ok == false {
				logrus.Debugf("User '%s' not found in config; tried to access '%s'", user, r.RequestURI)
				w.WriteHeader(403)
				return
			}

			// Every user is forced to their entrypoint
			if r.RequestURI == "/" {
				http.Redirect(w, r, config.Users[user].Entrypoint, http.StatusPermanentRedirect)
				return
			}

			// Check if the requested path is disallowed; if yes go to entrypoint
			for pathPart, pathConfig := range config.Users[user].Paths {
				if strings.Contains(r.RequestURI, pathPart) {
					if pathConfig.Allowed == false {
						http.Redirect(w, r, config.Users[user].Entrypoint, http.StatusPermanentRedirect)
						return
					}
				}
			}

			// Handle basicui access
			if strings.HasPrefix(r.RequestURI, "/basicui/app") {
				queryString := r.URL.Query()
				sitemap := queryString.Get("sitemap")
				if len(sitemap) == 0 {
					queryString.Set("sitemap", config.Users[user].Sitemaps.Default)
					r.URL.RawQuery = queryString.Encode()
				}
				if len(sitemap) != 0 && sitemap != config.Users[user].Sitemaps.Default {
					if len(config.Users[user].Sitemaps.Allowed) == 1 && config.Users[user].Sitemaps.Allowed[0] == "*" {
						goto serve
					}
					hit := false
					for _, allowedSitemap := range config.Users[user].Sitemaps.Allowed {
						if sitemap == allowedSitemap {
							hit = true
							break
						}
					}
					if hit == false {
						queryString.Set("sitemap", config.Users[user].Sitemaps.Default)
						r.URL.RawQuery = queryString.Encode()
					}
				}
			}
		}

	serve:
		proxy.ServeHTTP(w, r)
	})

	addr := *host + ":" + *port
	logrus.Infof("Serving at %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		logrus.WithError(err).Error("Listener failed")
		os.Exit(1)
	}
}
