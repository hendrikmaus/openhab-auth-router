# Authenticated Passthrough Example

Add openhab-auth-router as auth proxy between nginx and openhab
with basic authentication in place.

Use either `admin` or `demo` user and try the router in action.

## Required Tools

- docker
- docker-compose

## Services

This example runs a simple nginx, openhab in demo mode and open-auth-router
with basic authentication enabled.

The [config](./config.yaml), configures two users with different access to openhab.

- `admin`
  - access to everything
- `demo`
  - entrypoint is basic ui
  - `/start/index` is disallowed

## Authentication

Use the admin credentials:

- username: `admin`
- password: `admin`

Use the demo credentials:

- username: `demo`
- password: `demo`

## Start

```sh
docker-compose up -d
```

## Access OpenHAB

### Through Auth-Router

```sh
openhab http://localhost
```

- Go to OpenHAB in your browser
  - Login as `admin`
    - The `demo` user would not be allowed to access the start
      to setup the OpenHAB instance
  - Open the network console in the dev tools to inspect traffic
- Click on Demo mode
- Browse OpenHAB
  - Monitor network panel to see if there are any hidden issues
- Now login as `demo` user
  - Enter `http://demo:demo@localhost` to bust your admin session
- You should be redirected to `/basicui/app` and see the demo sitemap
  - Try to go to `http://localhost/start/index`
    and you should land in basic ui again

### Directly, Without Auth-Router

```sh
open http://localhost:8080
```

## Cleanup

```sh
docker-compose down
```