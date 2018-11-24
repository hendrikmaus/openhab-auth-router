package main

import (
	"flag"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/hendrikmaus/openhab-auth-router/util"
	_ "github.com/sanity-io/litter"
	log "github.com/sirupsen/logrus"
)

func main() {
	host := flag.String("host", "127.0.0.1", "Host to listen on")
	port := flag.String("port", "80", "Port to listen on")
	target := flag.String("target", "", "Address of your OpenHAB instance, e.g. 'http://openhab:8080'")
	logLevel := flag.String("log-level", "info", "Loglevel as in [error|warn|info|debug]")
	logType := flag.String("log-type", "auto", "Set the type of logging [human|human-color|machine|machine+color|auto]")
	flag.Parse()

	if len(*target) == 0 {
		log.Error("Please set '-target' to the address of your OpenHAB instance, e.g. 'http://openhab:8080'")
		os.Exit(1)
	}
	util.ConfigureLogger(logLevel, logType)

	remote, err := url.Parse(*target)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	proxy := httputil.NewSingleHostReverseProxy(remote)
	mux := http.NewServeMux()
	mux.HandleFunc("/liveness", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/readiness", func(w http.ResponseWriter, r *http.Request) {
		resp, err := http.Get(remote.String() + "/rest/")
		if err != nil || resp.StatusCode != http.StatusOK {
			log.Errorf("Readiness probe failed while trying to access '%s/rest/'", remote.String())
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		log.Debug("Readiness probe successful")
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Debug("Ingress")
		proxy.ServeHTTP(w, r)
	})

	addr := *host + ":" + *port
	log.Infof("Serving at %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
