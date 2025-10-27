#!/bin/sh

list() {
  xcaddy list-modules
}

# equivalent to using the hard-coded main.
build() {
  go install github.com/caddyserver/xcaddy/cmd/xcaddy@latest

  xcaddy build --output /x/sync/app/bin/caddy \
    --with github.com/caddy-dns/cloudflare \
    --with github.com/caddy-dns/googlecloud \
    --with github.com/caddy-dns/dnsoverhttps \
    --with github.com/caddy-dns/rfc2136 \
    --with http.reverse_proxy.transport.fastcgi \
    --with github.com/abiosoft/caddy-yaml
}

run() {
  /x/sync/app/bin/caddy run --config caddy.yaml --adapter yaml
}

$*
