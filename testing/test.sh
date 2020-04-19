#!/usr/bin/env bash
set -Eeuo pipefail

[[ ! -f "wait-for-it" ]] && \
  echo "downloading wait-for-it" && \
  wget -O wait-for-it https://raw.githubusercontent.com/vishnubob/wait-for-it/master/wait-for-it.sh && \
  chmod +x wait-for-it

# step 0 - run go build + fix permissions
echo "building openhab-auth-router"
docker run --rm \
  -v "$(pwd)/../":/go/src/github.com/hendrikmaus/openhab-auth-router \
  -w /go/src/github.com/hendrikmaus/openhab-auth-router \
  golang:1.13.8-buster \
  go build -o openhab-auth-router -mod=vendor /go/src/github.com/hendrikmaus/openhab-auth-router/main.go

echo "setting permissions"
mv ../openhab-auth-router . && \
docker run --rm \
  -v "$(pwd)":/workspace \
  -w /workspace \
  busybox:latest \
  chown "$(id -u)":"$(id -g)" openhab-auth-router

# step 1 - bootstrapping an openHAB container and wait for it to be up
echo "starting the test stack"
docker-compose up -d && trap "docker-compose down" EXIT

./wait-for-it localhost:8080 -- echo "openHAB container is up" && \
  ./wait-for-it localhost:80 -- echo "nginx container is up" && \
  ./wait-for-it localhost:9090 -- echo "openhab-auth-router container is up" && \
  echo "all containers running and listening"

printf "waiting for openHAB to start "
until curl -sL "http://localhost:8080/" | grep "Initial Setup" > /dev/null; do
  printf '.'
  sleep 2
done
echo " success"

# step 2 - trigger demo mode setup and wait for completion
printf "setup Demo mode in openHAB" && \
  curl -sL "http://localhost:8080/start/index?type=demo" > /dev/null && \
  echo " - success"

printf "wait for openHAB to setup "
until curl -sL "http://localhost:8080/basicui/app" | grep "Demo" > /dev/null; do
  printf '.'
  sleep 2
done
echo " success"

# step 3 - run test cases
echo "run test cases:"

echo "1. admin can see ui selection on /start/index"
curl -sL --fail --show-error -u admin:admin "http://localhost/start/index" | grep "Welcome to openHAB" > /dev/null

echo "2. admin can see sitemap 'admin'"
curl -sL --fail --show-error -u admin:admin "http://localhost/basicui/app?sitemap=admin" | grep "Admin" > /dev/null

echo "3. demo can not see ui selection on /start/index > redirects to /basicui/app?sitemap=demo"
curl -sL --fail --show-error -u demo:demo "http://localhost/start/index" | grep "Demo"> /dev/null

echo "4. demo user can not see admin sitemap > redirects to demo sitemap"
curl -sL --fail --show-error -u demo:demo "http://localhost/basicui/app?sitemap=admin" | grep "Demo" > /dev/null

echo "5. demo user can see demo sitemap"
curl -sL --fail --show-error -u demo:demo "http://localhost/basicui/app?sitemap=demo" | grep "Demo"> /dev/null

echo "6. demo user can see widgetoverview sitemap"
curl -sL --fail --show-error -u demo:demo "http://localhost/basicui/app?sitemap=widgetoverview" | grep "Widget Overview" > /dev/null

# cleanup
rm openhab-auth-router

echo "SUCCESS"
