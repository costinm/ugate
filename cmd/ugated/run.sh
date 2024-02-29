#!/bin/sh

# Wrapper for running in docker.
# Expects that it started as root - no separate init container,
# will set the default iptables egress if possible

PROXY_GID=1337

pwd
env
if [ -z "$PORT" ] ; then
  /usr/local/bin/iptables.sh
fi
exec su istio-proxy -s /usr/local/bin/ugate
