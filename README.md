# OpenHAB Auth Router

[![CircleCI](https://circleci.com/gh/hendrikmaus/openhab-auth-router/tree/master.svg?style=svg)](https://circleci.com/gh/hendrikmaus/openhab-auth-router/tree/master)

A quick solution to control sitemap access by authenticated users.

## Use Case

- You run one or more OpenHAB instances
- The instances are accessed through nginx
- Nginx is used to provide basic authentication
- You want to restrict user access to sitemaps

> Nginx is not a hard-requirement; any other service capable of your desired
> authentication and setting a heder called `X-Forwarded-Username` to the authenticated
> username will work fine

## Usage

> You might like to look at some of the [examples](./examples) to get started as well.

### Docker

The recommended way to run the router is using the official Docker image:

```sh
echo "passthrough: true" > ./config.yaml
docker run --rm \
  -v "$(pwd)/config.yaml:/usr/share/config.yaml" \
  -p 9090:9090 \
  hendrikmaus/openhab-auth-router:${TAG} \
    -host="0.0.0.0" \
    -port="9090" \
    -target="http://openhab:8080" \
    -config="/usr/share/config.yaml"
```

Now point your browser to port 9090 on the machine the container runs on.
You should be able to access your OpenHAB as usual.

> If you encounter issues, please open an issue on Github.

### Vanilla Binary

```sh
echo "passthrough: true" > ./config.yaml
./openhab-auth-router -host="127.0.0.1" -port="9090" -target="http://openhab:8080" -config="./config.yaml"
```

Now point your browser to port 9090 on the machine the binary runs on.
You should be able to access your OpenHAB as usual.

> If you encounter issues, please open an issue on Github.

## Setup

In order to ensure that the entirety of your system still functions once
the router is in place, it can be used as pure passthrough proxy. In this
mode it does not provide any restrictions; your setup should work as before.

Start by either pulling the official docker image or downloading the binary
for your platform from [github](https://github.com/hendrikmaus/openhab-auth-router/releases).

Depending on your OS, create a service that runs and maintains the router
process.

### Binary Via Systemd

The config file is expected to live at `/usr/share/openhab-auth-router/config.yaml`.

```txt
[Unit]
Description=openhab-auth-router

[Service]
Restart=always
ExecStart=/usr/bin/openhab-auth-router \
    -host="0.0.0.0" \
    -port="9090" \
    -target="http://openhab:8080" \
    -config="/usr/share/openhab-auth-router/config.yaml"

[Install]
WantedBy=multi-user.target
```

Place this `openhab-auth-router.service` file into `/etc/systemd/system`.

Then run `sudo systemctl enable openhab-auth-router.service`.

Finally run `sudo systemctl start openhab-auth-router.service` to start the router running.

### Managed by Docker

```sh
echo "passthrough: true" > ./config.yaml
docker run --restart always --name openhab-auth-router \
  -v "$(pwd)/config.yaml:/usr/share/config.yaml" \
  -p 9090:9090 \
  hendrikmaus/openhab-auth-router:${TAG} \
    -host="0.0.0.0" \
    -port="9090" \
    -target="http://openhab:8080" \
    -config="/usr/share/config.yaml"
```

### Docker Managed by Systemd

Ensure to replace `${TAG}` with the version you want to run.

The config file is expected to live at `/usr/share/openhab-auth-router/config.yaml`.

```txt
[Unit]
Description=openhab-auth-router
Requires=docker.service
After=docker.service

[Service]
Restart=always
ExecStart=/usr/bin/docker run --name=%n \
  -v /usr/share/openhab-auth-router/config.yaml:/usr/share/config.yaml \
  -p 9090:9090 \
  hendrikmaus/openhab-auth-router:${TAG} \
    -host="0.0.0.0" \
    -port="9090" \
    -target="http://openhab:8080" \
    -config="/usr/share/config.yaml"
ExecStop=/usr/bin/docker stop -t 2 %n ; /usr/bin/docker rm -f %n

[Install]
WantedBy=multi-user.target
```

Place this `openhab-auth-router.service` file into `/etc/systemd/system`.

Then run `sudo systemctl enable openhab-auth-router.service`.

Finally run `sudo systemctl start openhab-auth-router.service` to start the router running.

### Kubernetes

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: openhab-auth-router-config
  namespace: openhab
  labels:
    app: openhab-auth-router
data:
  config.yaml: |-
    passthrough: true
---
apiVersion: v1
kind: Service
metadata:
  name: openhab-auth-router
  namespace: openhab
  labels:
    app: openhab-auth-router
spec:
  selector:
    app: openhab-auth-router
  ports:
  - name: http-proxy
    port: 8080
    targetPort: http-proxy
    protocol: HTTP
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: openhab-auth-router
  namespace: openhab
spec:
  selector:
    matchLabels:
      app: openhab-auth-router
  replicas: 1
  template:
    metadata:
      labels:
        app: openhab-auth-router
    spec:
      containers:
      - name: openhab-auth-router
        image: hendrikmaus/openhab-auth-router:latest
        command:
          - openhab-auth-router
          - -host="0.0.0.0"
          - -port="8080"
          - -target="http://openhab.default.svc.cluster.local"
          - -config="/usr/share/config.yaml"
          - -log-level="debug"
          - -log-type="human"
        volumeMounts:
        - name: config-volume
          mountPath: /usr/share/
        ports:
        - name: http-proxy
          containerPort: 8080
        readinessProbe:
          httpGet:
            path: /readiness
            port: 8080
          initialDelaySeconds: 5
          timeoutSeconds: 5
        livenessProbe:
          httpGet:
            path: /liveness
            port: 8080
          initialDelaySeconds: 5
          timeoutSeconds: 5
      volumes:
      - name: config-volume
        configMap:
          name: openhab-auth-router-config
```

Asserting the health of the router and connection to the target:

```sh
curl -v host:port/liveness
# should respond with HTTP 200 OK

curl -v host:port/readiness
# should respond with HTTP 200 OK
```

- liveness
  - checks the pure liveness of the process
- readiness
  - determine readiness for traffic
  - determine healthy connection to target system

Now point your nginx to the router instead of the OpenHAB instance:

```txt
    location / {
        proxy_pass                              http://openhab-auth-router/;
        proxy_redirect                          off;
        proxy_http_version                      1.1;
        proxy_set_header Host                   $http_host;
        proxy_set_header Upgrade                $http_upgrade;
        proxy_set_header Connection             "upgrade";
        proxy_set_header X-Real-IP              $remote_addr;
        proxy_set_header X-Forwarded-For        $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto      $scheme;

        ## Authentication; leave this out for the first test
        # proxy_set_header X-Forwarded-Username   $remote_user;
        # auth_basic                              "Username and Password Required";
        # auth_basic_user_file                    /etc/nginx/.htpasswd;
    }
```

Restart nginx `sudo service nginx reload` and try to access your OpenHAB instance as usual.

## Development And Contribution

### Using The Makefile

The Makefile is self-documenting, simply run `make` to see the help.

### Specific Golang Version Required

Since OpenHAB's BasicUI uses WebSockets, we need a feature of golang
which is unreleased right now (2018-11-24), but the [feature][1] is already merged
into golang master.

Hence you need a golang version compiled at `ee55f0856a3f1fed5d8c15af54c40e4799c2d32f` or
newer until golang 1.12 is released.

To solve this issue, the project provides the means to build the required
golang version in a docker container:

> Actually, I provide that very commit with `hendrikmaus/golang` on dockerhub

```sh
make builder-golang
```

The commit defaults to the afforementioned, but can be overridden by setting `GO_COMMIT`.

To build the docker image from the based golang image, run:

```sh
make pkg-docker
```

### Releasing

The router is distributed for many systems, the Makefile provides the means to build
all required packages:

```sh
IMAGE_TAG={version} make pkg-all
```

Now the `./dist` folder is populated and the docker image is built locally.

To push the docker image:

```sh
IMAGE_TAG={version} make pkg-docker-push
```

[1]:https://github.com/golang/go/commit/ee55f0856a3f1fed5d8c15af54c40e4799c2d32f
