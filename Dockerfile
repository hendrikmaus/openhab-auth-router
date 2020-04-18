FROM golang:buster as builder

COPY . /go/src/github.com/hendrikmaus/openhab-auth-router
WORKDIR /go/src/github.com/hendrikmaus/openhab-auth-router
RUN make build

FROM gcr.io/distroless/base-debian10
COPY --from=builder /go/src/github.com/hendrikmaus/openhab-auth-router/openhab-auth-router /usr/local/bin/openhab-auth-router
