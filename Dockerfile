#FROM golang:latest AS build
FROM golang:alpine AS build
#RUN apk add --no-cache git

RUN env

# Default is /go directory, which is set a GOPATH
# That doesn't work with go.mod
WORKDIR /ws

ENV GO111MODULE=on
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOPROXY=https://proxy.golang.org

COPY . .

# Runs in /go directory
RUN go build -a -gcflags='all=-N -l' -ldflags '-extldflags "-static"' \
  -o ugate ./cmd/ugate

FROM alpine:latest

## Same base as Istio debug
#FROM ubuntu:bionic AS wps
# Or distroless
#FROM docker.io/istio/base:default AS wps
#RUN apt-get update && apt install less net-tools

COPY --from=build /ws/ugate /usr/local/bin/ugate
COPY --from=build /ws/cmd/ugate/iptables.sh /usr/local/bin/
COPY --from=build /ws/cmd/ugate/run.sh /usr/local/bin/
#COPY --from=build /ws/dlv /usr/local/bin/dlv

RUN apk add iptables ip6tables make &&\
    mkdir -p /var/lib/istio && \
    addgroup -g 1337 istio-proxy && \
    adduser -S -G istio-proxy istio-proxy -u 1337 && \
    mkdir -p /var/lib/istio && \
    chown -R 1337:1337 /var/lib/istio

WORKDIR /var/lib/istio
RUN mkdir -p /etc/certs && \
    mkdir -p /etc/istio/proxy && \
    mkdir -p /etc/istio/config && \
    mkdir -p /var/lib/istio/envoy && \
    mkdir -p /var/lib/istio/config && \
    mkdir -p /var/lib/istio/proxy && \
    chown -R 1337 /etc/certs /etc/istio /var/lib/istio

EXPOSE 15007
EXPOSE 8080
EXPOSE 15009
EXPOSE 15003

ENV PORT=8080

# Defaults
#COPY ./var/lib/istio /var/lib/istio/
#USER 5228:5228
#ENTRYPOINT /usr/local/bin/ugate
ENTRYPOINT /usr/local/bin/run.sh
