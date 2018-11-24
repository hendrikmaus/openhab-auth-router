# Authenticated Passthrough Example

Add openhab-auth-router as passthrough proxy between nginx and openhab
with basic authentication in place.

## Required Tools

- docker
- docker-compose

## Services

This example runs a simple nginx, openhab in demo mode and open-auth-router
with basic authentication enabled. Other than that, the router runs as passthrough.

With this example you should be able to use the OpenHAB demo application through
the proxy without any errors.

All services expose ports to offer maximum testing flexibility.

## Authentication

Use the demo credentials:

- username: `demo`
- password: `demo`

## Start

```sh
docker-compose up -d
```

## Access OpenHAB

### Thorugh Auth-Router

```sh
openhab http://localhost
```

- Go to OpenHAB in your browser
  - Open the network console in the dev tools to inspect traffic
- Click on Demo mode
- Browse OpenHAB
  - Monitor network panel to see if there are any hidden issues
- You may also want to access the rest api

### Directly, Without Auth-Router

```sh
open http://localhost:8080
```

## Cleanup

```sh
docker-compose down
```