# Authenticated Passthrough Example

Add openhab-auth-router as auth proxy between nginx and openHAB
with basic authentication in place.

Use either `admin` or `demo` user and try the router in action.

## Required Tools

- docker
- docker-compose

## Services

This example runs a simple nginx, openHAB and open-auth-router
with basic authentication enabled.

The [config](./config.yaml), configures two users with different access to openHAB.

- `admin`
  - access to everything
  - start with this user to kick off openHAB demo mode
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
docker-compose up openhab-auth-router-build && docker-compose up -d
```

## Access OpenHAB

### Through Auth-Router

- Go to openHAB in your browser, private window (it can take up to a few minutes to be available)
  - Login as `admin`
    - The `demo` user would not be allowed to access the start
      to setup the openHAB instance
  - Open the network console in the dev tools to inspect traffic
- Click on Demo mode
- Browse openHAB
  - Monitor network panel to see if there are any hidden issues
- Now login as `demo` user
  - Open a new private window or
  - Enter `http://demo:demo@localhost` to bust your admin session
    - does not work reliably for some
- You should be redirected to `/basicui/app` and see the demo sitemap
  - Try to go to `http://localhost/start/index`
    and you should land in basic ui again
  - Try to access other sitemaps as well
    - `widgetoverview` should work
    - `admin` not

### Directly, Without Auth-Router

```sh
open http://localhost:8080
```

## Cleanup

```sh
docker-compose down
```
