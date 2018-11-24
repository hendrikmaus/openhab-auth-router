# OpenHAB Auth Router

A quick solution to control sitemap access by authenticated users.

## Use Case

- You run one or more OpenHAB instances
- The instances are accessed through nginx
- Nginx is used to provide basic authentication
- You want to restrict user access to sitemaps

## Usage

> You might like to look at some of the [examples](./examples) to get started as well.

### Docker

The recommended way to run the router is using the official Docker image:

```sh
hendrikmaus/openhab-auth-router:${TAG}
```

### Vanilla Binary

```sh
./openhab-auth-router -host="127.0.0.1" -port="9090" -target="http://openhab:8080"
```

Now point your browser to [localhost:8080](http://localhost:8080).

## Setup

In order to ensure that the entirety of your system still functions once
the router is in place, it can be used as pure passthrough proxy. In this
mode it does not provide any restrictions; your setup should work as before.

Start by either pulling the official docker image or downloading the binary
for your platform from [github](https://github.com/hendrikmaus/openhab-auth-router/releases).

Depending on your OS, create a service that runs and maintains the router
process.

E.g. systemd:
TODO: TBD

E.g. managed by docker:
TODO: TBD

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
        # proxy_set_header X-Forwarded-Username   $user;
        # auth_basic                              "Username and Password Required";
        # auth_basic_user_file                    /etc/nginx/.htpasswd;
    }
```

TODO: update with basic auth and header set for router

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
