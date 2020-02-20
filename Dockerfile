FROM golang:1.13.8 as builder

COPY . /go/src/github.com/hendrikmaus/openhab-auth-router
WORKDIR /go/src/github.com/hendrikmaus/openhab-auth-router
RUN go build -tags netgo -ldflags "-X 'main.Version=${IMAGE_TAG}'" -mod=vendor

FROM alpine

COPY --from=builder /go/src/github.com/hendrikmaus/openhab-auth-router/openhab-auth-router /usr/local/bin
