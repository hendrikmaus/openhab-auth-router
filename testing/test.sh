#!/usr/bin/env bash
set -eu -o pipefail
set -x

if [ ! -f "wait-for-it" ]; then
  wget -O wait-for-it https://raw.githubusercontent.com/vishnubob/wait-for-it/master/wait-for-it.sh
  chmod +x wait-for-it
fi

# step 0 - run go build + fix permissions
docker run --rm \
  -v "$(pwd)/../":/go/src/github.com/hendrikmaus/openhab-auth-router \
  -w /go/src/github.com/hendrikmaus/openhab-auth-router \
  golang:1.13.8-buster \
  go build -o openhab-auth-router -mod=vendor /go/src/github.com/hendrikmaus/openhab-auth-router/main.go

mv ../openhab-auth-router .
docker run --rm \
  -v "$(pwd)":/workspace \
  -w /workspace \
  busybox:latest \
  chown "$(id -u)":"$(id -g)" openhab-auth-router

# step 1 - bootstrapping an openHAB container and wait for it to be up
docker-compose up -d
trap "docker-compose down" EXIT

# wait for openhab itself to listen on port 8080
./wait-for-it localhost:8080 -- echo "openHAB container is up"

# wait for nginx to listen on port 80
./wait-for-it localhost:80 -- echo "nginx container is up"

# wait for openhab-auth-router to listen on port 9090
./wait-for-it localhost:9090 -- echo "openhab-auth-router container is up"
sleep 30
curl -sL "http://localhost:8080/" | grep "Initial Setup"

# step 2 - trigger demo mode setup and wait for completion
curl -sL "http://localhost:8080/start/index?type=demo"
sleep 60
curl -sL "http://localhost:8080/basicui/app" | grep "Demo"

# step 3 - run test cases

# admin can see ui selection on /start/index
curl -sL -u admin:admin "http://localhost:80/start/index" | grep "Welcome to openHAB"

# admin can see sitemap "admin"
curl -sL -u admin:admin "http://localhost/basicui/app?sitemap=admin" | grep "Admin"

# demo can not see ui selection on /start/index > redirects to /basicui/app?sitemap=demo
curl -sL -u demo:demo "http://localhost/start/index" | grep "Demo"

# demo user can not see admin sitemap > redirects to demo sitemap
curl -sL -u demo:demo "http://localhost/basicui/app?sitemap=admin" | grep "Demo"

# demo user can see demo sitemap
curl -sL -u demo:demo "http://localhost/basicui/app?sitemap=demo" | grep "Demo"

# demo user can see widgetoverview sitemap
curl -sL -u demo:demo "http://localhost/basicui/app?sitemap=widgetoverview" | grep "Widget Overview"

# cleanup
rm openhab-auth-router
