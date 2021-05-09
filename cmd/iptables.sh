#!/bin/sh

# Simplified istio iptables
# Use a different GID to run iperf3 or tests.

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
# PROXY_GROUP=costin INBOUND_PORTS_INCLUDE=5201 OUTBOUND_PORTS_INCLUDE=5201

OUTBOUND_CAPTURE_PORT=${OUTBOUND_CAPTURE_PORT:-15001}
INBOUND_CAPTURE_PORT=${INBOUND_CAPTURE_PORT:-15006}

set -x

# Default mode is Istio compatible capture
if [ -z "${IN}" ] ; then
  INBOUND_PORTS_EXCLUDE=${INBOUND_PORTS_EXCLUDE:-"-"}
  OUTBOUND_PORTS_EXCLUDE=${OUTBOUND_PORTS_EXCLUDE:-"-"}
else
  # Default opt-in list of capture.
  # May be set to "-" to not capture anything.
  # If _PORTS_EXCLUDE is set, all ports except excluded are captured.
  INBOUND_PORTS_INCLUDE=${INBOUND_PORTS_INCLUDE:-${IN}}
  OUTBOUND_PORTS_INCLUDE=${OUTBOUND_PORTS_INCLUDE:-${OUT}}
fi


# If INCLUDE is set, only those ports are captured, else
# all ports except EXCLUDE are captured

if [ -z "${PROXY_GID}" ] ; then
  PROXY_GID=$(id -g "${PROXY_GROUP:-istio-proxy}")
fi

ipt_clean() {
  # Remove the old chains, to generate new configs.
  ${IPT} -t nat -D PREROUTING -p tcp -j ISTIO_INBOUND 2>/dev/null
  ${IPT} -t mangle -D PREROUTING -p tcp -j ISTIO_INBOUND 2>/dev/null
  ${IPT} -t nat -D OUTPUT -p tcp -j ISTIO_OUTPUT 2>/dev/null

  # Flush and delete the istio chains.
  ${IPT} -t nat -F ISTIO_OUTPUT 2>/dev/null
  ${IPT} -t nat -X ISTIO_OUTPUT 2>/dev/null
  ${IPT} -t nat -F ISTIO_INBOUND 2>/dev/null
  ${IPT} -t nat -X ISTIO_INBOUND 2>/dev/null

  # Must be last, the others refer to it
  ${IPT} -t nat -F ISTIO_REDIRECT 2>/dev/null
  ${IPT} -t nat -X ISTIO_REDIRECT 2>/dev/null
  ${IPT} -t nat -F ISTIO_IN_REDIRECT 2>/dev/null
  ${IPT} -t nat -X ISTIO_IN_REDIRECT 2>/dev/null
}

ipt_out() {
  # Create a new chain for redirecting outbound traffic to the common Envoy port.
  # In both chains, '-j RETURN' bypasses Envoy and '-j ISTIO_REDIRECT'
  # redirects to Envoy.
  ${IPT} -t nat -N ISTIO_REDIRECT
  ${IPT} -t nat -A ISTIO_REDIRECT -p tcp -j REDIRECT --to-port "${OUTBOUND_CAPTURE_PORT}"

  # Create a new chain for selectively redirecting outbound packets to Envoy.
  ${IPT} -t nat -N ISTIO_OUTPUT
  # Jump to the ISTIO_OUTPUT chain from OUTPUT chain for all tcp traffic.
  ${IPT} -t nat -A OUTPUT -p tcp -j ISTIO_OUTPUT

  if [ ${IPT} == "iptables" ] ; then
    # 127.0.0.6 is bind connect from inbound passthrough cluster
    ${IPT} -t nat -A ISTIO_OUTPUT -o lo -s 127.0.0.6/32 -j RETURN
    # Skip redirection for Envoy-aware applications and
    # container-to-container traffic both of which explicitly use
    # localhost.
    ${IPT} -t nat -A ISTIO_OUTPUT -d 127.0.0.1/32 -j RETURN
  else
    ${IPT} -t nat -A ISTIO_OUTPUT -d ::1/32 -j RETURN
    # Capture FD00 - VIP for mesh nodes
    ${IPT} -t nat -A ISTIO_OUTPUT -d fd00::/16 -j ISTIO_REDIRECT
  fi

  # Avoid infinite loops. Don't redirect Envoy traffic directly back to
  # Envoy for non-loopback traffic.
  ${IPT} -t nat -A ISTIO_OUTPUT -m owner --gid-owner "${PROXY_GID}" -j RETURN

  if [ -n "${OUTBOUND_PORTS_EXCLUDE}" ]; then
    # BTS port is direct.
    ${IPT} -t nat -A ISTIO_OUTPUT -p tcp --dport 15007 -j RETURN
    if [ "${OUTBOUND_PORTS_EXCLUDE}" != "-" ]; then
      IFS=,
      for port in $OUTBOUND_PORTS_EXCLUDE ; do
        ${IPT} -t nat -A ISTIO_OUTPUT -p tcp --dport "${port}" -j RETURN
      done
    fi
    # Everything else
    ${IPT} -t nat -A ISTIO_OUTPUT -p tcp -j ISTIO_REDIRECT
  else
      # For probing iptables
      ${IPT} -t nat -A ISTIO_OUTPUT -p tcp --dport 15201 -j ISTIO_REDIRECT
      if [ -n "${OUTBOUND_PORTS_INCLUDE}" ]; then
        IFS=,
        for port in ${OUTBOUND_PORTS_INCLUDE} ; do
          ${IPT} -t nat -A ISTIO_OUTPUT -p tcp --dport "${port}" -j ISTIO_REDIRECT
        done
      fi
  fi
}

ipt_in() {
  # Use this chain also for redirecting inbound traffic to the common Envoy port
  ${IPT} -t nat -N ISTIO_IN_REDIRECT
  ${IPT} -t nat -A ISTIO_IN_REDIRECT -p tcp -j REDIRECT --to-port "${INBOUND_CAPTURE_PORT}"

  ${IPT} -t nat -N ISTIO_INBOUND
  ${IPT} -t nat -A PREROUTING ${IN_IF:-} -p tcp -j ISTIO_INBOUND

  # Istio uses * to indicate all capture.
  if [ -n "${INBOUND_PORTS_EXCLUDE}" ]; then
    ${IPT} -t nat -A ISTIO_INBOUND -p tcp --dport 22 -j RETURN
    ${IPT} -t nat -A ISTIO_INBOUND -p tcp --dport 15007 -j RETURN
    if [ "${INBOUND_PORTS_EXCLUDE}" != "-" ]; then
      IFS=,
      for port in ${INBOUND_PORTS_EXCLUDE} ; do
        ${IPT} -t nat -A ISTIO_INBOUND -p tcp --dport "${port}" -j RETURN
      done
    fi
    ${IPT} -t nat -A ISTIO_INBOUND -p tcp -j ISTIO_IN_REDIRECT
  else
    if [ "${INBOUND_PORTS_INCLUDE}" != "-" ] ; then
      IFS=,
      for port in ${INBOUND_PORTS_INCLUDE}; do
        ${IPT} -t nat -A ISTIO_INBOUND -p tcp --dport "${port}" -j ISTIO_IN_REDIRECT
      done
    fi
  fi
}

IPT=iptables ipt_clean
IPT=iptables ipt_in
IPT=iptables ipt_out

IPT=ip6tables ipt_clean
IPT=ip6tables ipt_in
IPT=ip6tables ipt_out

