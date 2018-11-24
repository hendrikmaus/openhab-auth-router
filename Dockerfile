ARG GO_COMMIT

FROM hendrikmaus/golang:$GO_COMMIT as builder

RUN apk update && \
    apk add make

COPY . /go/src/github.com/hendrikmaus/openhab-auth-router
WORKDIR /go/src/github.com/hendrikmaus/openhab-auth-router
RUN make build

FROM alpine

COPY --from=builder /go/src/github.com/hendrikmaus/openhab-auth-router/openhab-auth-router /usr/local/bin