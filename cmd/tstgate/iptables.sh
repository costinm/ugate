#!/bin/bash

# Simplified istio iptables, for single port
# Use a different GID to run the iperf3 or tests.

# Defaults:
# - capture all in and out traffic, unless:
#    - {INBOUND|OUTBOUND}_PORTS_INCLUDE - only those ports will be captured
#    - {INBOUND|OUTBOUND}_PORTS_EXCLUDE - if INCLUDE is not set, everything except those ports
# To turn off, set *_PORTS_INCLUDE to "-".

# Ranges are not supported: istio used them as a workaround
# before transparent proxy worked efficiently.

# Capturing with tproxy can be done with separate script, should
# not be mixed in.


# For testing iperf3, use:
# PROXY_GROUP=costin INBOUND_PORT_INCLUDE=5201 OUTBOUND_PORT_INCLUDE=5201

# also ENVOY_PORT - Istio doesn't have tests with othre ports
# so can be assumed to be fixed.
OUTBOUND_CAPTURE_PORT=${OUTBOUND_CAPTURE_PORT:-15001}
INBOUND_CAPTURE_PORT=${INBOUND_CAPTURE_PORT:-15006}

# If INCLUDE is set, only those ports are captured, else
# all ports except EXCLUDE are captured

if [ -z "${PROXY_GID}" ]; then
  PROXY_GID=$(id -g "${PROXY_GROUP:-istio-proxy}")
fi

function ipt_clean() {
  # Remove the old chains, to generate new configs.
  iptables -t nat -D PREROUTING -p tcp -j ISTIO_INBOUND 2>/dev/null
  iptables -t mangle -D PREROUTING -p tcp -j ISTIO_INBOUND 2>/dev/null
  iptables -t nat -D OUTPUT -p tcp -j ISTIO_OUTPUT 2>/dev/null

  # Flush and delete the istio chains.
  iptables -t nat -F ISTIO_OUTPUT 2>/dev/null
  iptables -t nat -X ISTIO_OUTPUT 2>/dev/null
  iptables -t nat -F ISTIO_INBOUND 2>/dev/null
  iptables -t nat -X ISTIO_INBOUND 2>/dev/null

  # Must be last, the others refer to it
  iptables -t nat -F ISTIO_REDIRECT 2>/dev/null
  iptables -t nat -X ISTIO_REDIRECT 2>/dev/null
  iptables -t nat -F ISTIO_IN_REDIRECT 2>/dev/null
  iptables -t nat -X ISTIO_IN_REDIRECT 2>/dev/null
}

function ipt_out() {
  # Create a new chain for redirecting outbound traffic to the common Envoy port.
  # In both chains, '-j RETURN' bypasses Envoy and '-j ISTIO_REDIRECT'
  # redirects to Envoy.
  iptables -t nat -N ISTIO_REDIRECT
  iptables -t nat -A ISTIO_REDIRECT -p tcp -j REDIRECT --to-port "${OUTBOUND_CAPTURE_PORT}"
  # Create a new chain for selectively redirecting outbound packets to Envoy.
  iptables -t nat -N ISTIO_OUTPUT

  # Jump to the ISTIO_OUTPUT chain from OUTPUT chain for all tcp traffic.
  iptables -t nat -A OUTPUT -p tcp -j ISTIO_OUTPUT

  # 127.0.0.6 is bind connect from inbound passthrough cluster
  iptables -t nat -A ISTIO_OUTPUT -o lo -s 127.0.0.6/32 -j RETURN
  for gid in ${PROXY_GID}; do
    # Avoid infinite loops. Don't redirect Envoy traffic directly back to
    # Envoy for non-loopback traffic.
    iptables -t nat -A ISTIO_OUTPUT -m owner --gid-owner "${gid}" -j RETURN
  done
  # Skip redirection for Envoy-aware applications and
  # container-to-container traffic both of which explicitly use
  # localhost.
  iptables -t nat -A ISTIO_OUTPUT -d 127.0.0.1/32 -j RETURN

  if [ -n "${OUTBOUND_PORTS_INCLUDE}" ]; then
      if ! [[ ${OUTBOUND_PORTS_INCLUDE} == "-" ]]; then
        IFS=, read -a fields <<<"${OUTBOUND_PORTS_INCLUDE}"
        for port in "${fields[@]}"; do
          iptables -t nat -A ISTIO_OUTPUT -p tcp --dport "${port}" -j ISTIO_REDIRECT
        done
      fi
  else
    if [ -n "${OUTBOUND_PORTS_EXCLUDE}" ]; then
      IFS=, read -a fields <<<"${OUTBOUND_PORTS_EXCLUDE}"
      for port in "${fields[@]}"; do
        iptables -t nat -A ISTIO_OUTPUT -p tcp --dport "${port}" -j RETURN
      done
    fi
    # Everything else
    iptables -t nat -A ISTIO_OUTPUT -p tcp -j ISTIO_REDIRECT
  fi
}

function ipt_in() {
  # Use this chain also for redirecting inbound traffic to the common Envoy port
  iptables -t nat -N ISTIO_IN_REDIRECT
  iptables -t nat -A ISTIO_IN_REDIRECT -p tcp -j REDIRECT --to-port "${INBOUND_CAPTURE_PORT}"

  iptables -t nat -N ISTIO_INBOUND
  iptables -t nat -A PREROUTING -p tcp -j ISTIO_INBOUND

  # Istio uses * to indicate all capture
  if [ -n "${INBOUND_PORTS_INCLUDE}" ]; then
    if ! [[ ${INBOUND_PORTS_INCLUDE} == "-" ]]; then
      IFS=, read -a fields <<<"${INBOUND_PORTS_INCLUDE}"
      for port in "${fields[@]}"; do
        iptables -t nat -A ISTIO_INBOUND -p tcp --dport "${port}" -j ISTIO_IN_REDIRECT
      done
    fi
  else
    iptables -t nat -A ISTIO_INBOUND -p tcp --dport 22 -j RETURN
    if [ -n "${INBOUND_PORTS_EXCLUDE}" ]; then
      IFS=, read -a fields <<<"${INBOUND_PORTS_EXCLUDE}"
      for port in "${fields[@]}"; do
        iptables -t nat -A ISTIO_INBOUND -p tcp --dport "${port}" -j RETURN
      done
    fi
    iptables -t nat -A ISTIO_INBOUND -p tcp -j ISTIO_IN_REDIRECT
  fi
}

ipt_clean

#set -x

ipt_in
ipt_out

