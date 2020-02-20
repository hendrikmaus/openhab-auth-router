# Simple Passthrough Example

Add openhab-auth-router as passthrough proxy between nginx and openHAB.

## Required Tools

- docker
- docker-compose

## Services

This example runs a simple nginx, openHAB in demo mode and open-auth-router
without any restrictions - a pure [passthrough](./config.yaml).

With this example you should be able to use the openHAB demo application through
the proxy without any errors.

All services expose ports to offer maximum testing flexibility.

## Start

```sh
docker-compose up openhab-auth-router-build && docker-compose up -d
```

## Access OpenHAB

### Through Auth-Router

- Go to openHAB in your browser (it can take up to a few minutes to be available)
  - Open the network console in the dev tools to inspect traffic
- Click on Demo mode
- Browse openHAB
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
