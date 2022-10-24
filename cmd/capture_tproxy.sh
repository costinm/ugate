#!/usr/bin/env bash

#
# Once: ipt_init ( can be reverted with ipt_reset)
# - will set common hooks and rules
#
# tunup/tundown - setup dmesh1 and prepare for capture

# Create common tables for dmesh.
# DMESH + MANGLE|NAT + PRE|POST|OUT
# ISTIO_REDIRECT
# ISTIO_DIVERT - set mark
# ISTIO_TPROXY - set port, mark

PROXY_GID="1337"
#addgroup --gid 1337 istio-proxy


function ipt_init() {
    # When using TPROXY, create a new chain for routing all inbound traffic to
    # Envoy. Any packet entering this chain gets marked with the ${INBOUND_TPROXY_MARK} mark,
    # so that they get routed to the loopback interface in order to get redirected to Envoy.
    # In the ISTIO_INBOUND chain, '-j ISTIO_DIVERT' reroutes to the loopback
    # interface.
    # Mark all inbound packets.
    iptables -t mangle -N ISTIO_DIVERT

    # TODO: only TCP ?
    iptables -t mangle -A ISTIO_DIVERT -j MARK --set-mark 1337
    iptables -t mangle -A ISTIO_DIVERT -j ACCEPT

    # Create a new chain for redirecting inbound traffic to the common Envoy
    # port.
    # In the ISTIO_INBOUND chain, '-j RETURN' bypasses Envoy and
    # '-j ISTIO_TPROXY' redirects to Envoy.
    iptables -t mangle -N ISTIO_TPROXY
    iptables -t mangle -A ISTIO_TPROXY ! -d 127.0.0.1/32 -p tcp -j TPROXY --tproxy-mark 1337/0xffffffff --on-port 15006

    # Route all packets marked in chain ISTIO_DIVERT using routing table ${INBOUND_TPROXY_ROUTE_TABLE}.
    ip -f inet rule add fwmark 1337 lookup 133
    # In routing table ${INBOUND_TPROXY_ROUTE_TABLE}, create a single default rule to route all traffic to
    # the loopback interface.
    ip -f inet route add local default dev lo table 133

    iptables -t mangle -N ISTIO_INBOUND
    # Bypass tproxy for established sockets
    iptables -t mangle -A ISTIO_INBOUND -p tcp -m socket -j ISTIO_DIVERT || echo "No socket match support"
    iptables -t mangle -A PREROUTING -p tcp -j ISTIO_INBOUND

    iptables -t nat -N ISTIO_OUTPUT
    iptables -t nat -A OUTPUT -p tcp -j ISTIO_OUTPUT

    iptables -t filter -N DMESH_FILTER_IN
    iptables -t filter -A INPUT -j DMESH_FILTER_IN

    iptables -t filter -N DMESH_FILTER_OUT
    iptables -t filter -A OUTPUT -j DMESH_FILTER_OUT

    iptables -t filter -N DMESH_FILTER_FWD
    iptables -t filter -A FORWARD -j DMESH_FILTER_FWD

    iptables -t nat -N DMESH_TCP_PRE
    iptables -t nat -A PREROUTING -p tcp -j DMESH_TCP_PRE

    iptables -t mangle -N DMESH_MANGLE_PRE
    iptables -t mangle -A PREROUTING -j DMESH_MANGLE_PRE

    iptables -t mangle -N DMESH_MANGLE_POST
    iptables -t mangle -A POSTROUTING  -j DMESH_MANGLE_POST

    iptables -t mangle -N DMESH_MANGLE_OUT
    iptables -t mangle -A OUTPUT  -j DMESH_MANGLE_OUT

    # Per user/gid mark must be set on OUTPUT (or POSTROUTE). We set a mark.
    #GID=$(id -g costin)
    iptables -t mangle -A DMESH_MANGLE_OUT -m owner --gid-owner ${PROXY_GID} -j MARK --set-mark 12
    iptables -t mangle -A DMESH_MANGLE_OUT -m owner --gid-owner ${PROXY_GID} -j RETURN

    #iptables -t mangle -A ISTIO_DIVERT -j LOG -p udp --log-prefix "dmesh-divert-udp"
    #iptables -t mangle -A ISTIO_DIVERT -j LOG -p tcp --log-prefix "dmesh-divert-tcp"

    #iptables -t mangle -A ISTIO_TPROXY -j LOG --log-prefix "dmesh-tproxy"


    # In routing table ${INBOUND_TPROXY_ROUTE_TABLE}, create a single default rule to route all traffic to
    # the loopback interface.

    #ip rule list
    #ip -s -d -a route list table all | grep -v "table local" |grep -v "table main"

}

# Opposite of init - delete the interception
function ipt_clean() {
    # Flush and delete the istio chains.
    iptables -t nat -F ISTIO_OUTPUT 2>/dev/null
    iptables -t mangle -F ISTIO_INBOUND 2>/dev/null

    iptables -t mangle -F DMESH_MANGLE_POST
    iptables -t mangle -F DMESH_MANGLE_PRE

    iptables -t filter -F DMESH_FILTER_IN
    iptables -t filter -F DMESH_FILTER_OUT
    iptables -t filter -F DMESH_FILTER_FWD
}

# Clean and delete all tables created by DMESH
function ipt_reset() {
    ipt_clean

    iptables -t mangle -F ISTIO_DIVERT 2>/dev/null
    iptables -t mangle -F ISTIO_TPROXY 2>/dev/null
    iptables -t mangle -F DMESH_MANGLE_OUT
    iptables -t nat -F DMESH_TCP_PRE

    ip rule delete from all fwmark 1337 lookup 133
    ip -f inet route del local default dev lo table 133

    # Make sure we're in clean state
    iptables -t mangle  -D POSTROUTING -j DMESH_MANGLE_POST
    iptables -t mangle  -D PREROUTING -j DMESH_MANGLE_PRE
    iptables -t mangle  -D OUTPUT -j DMESH_MANGLE_OUT
    iptables -t mangle -D PREROUTING -p tcp -j ISTIO_INBOUND
    iptables -t nat -D OUTPUT -p tcp -j ISTIO_OUTPUT

    iptables -t filter -D FORWARD -j DMESH_FILTER_FWD
    iptables -t filter -D INPUT -j DMESH_FILTER_IN
    iptables -t filter -D OUTPUT -j DMESH_FILTER_OUT
    iptables -t nat -D PREROUTING -p tcp -j DMESH_TCP_PRE

    iptables -t nat -X DMESH_TCP_PRE

    iptables -t mangle -X ISTIO_TPROXY
    iptables -t mangle -X ISTIO_DIVERT
    iptables -t mangle -X ISTIO_INBOUND
    iptables -t nat -X ISTIO_OUTPUT

    iptables -t mangle -X DMESH_MANGLE_POST
    iptables -t mangle -X DMESH_MANGLE_PRE
    iptables -t mangle -X DMESH_MANGLE_OUT
    iptables -t filter -X DMESH_FILTER_IN
    iptables -t filter -X DMESH_FILTER_OUT
    iptables -t filter -X DMESH_FILTER_FWD
}

# Simplified istio iptables script.
# Constants:
# - ISTIO_TPROXY_MARK=1337
# - route table 133
# - proxy port 15001 for outbound, 15004 for inbound
#

# Based on INBOUND_PORT_INCLUDE/EXCLUDE, set capture for in ports.
# This is redirected from PREROUTING table
function istio_iptable_input() {
    iptables -t mangle -F ISTIO_INBOUND

    if [ "${INBOUND_PORTS_INCLUDE}" == "*" ]; then

        # Makes sure SSH is not redirected
        iptables -t mangle -A ISTIO_INBOUND -p tcp --dport 22 -j RETURN

        # Apply any user-specified port exclusions.
        if [ -n "${INBOUND_PORTS_EXCLUDE}" ]; then
          for port in ${INBOUND_PORTS_EXCLUDE}; do
            iptables -t mangle -A ISTIO_INBOUND --dport "${port}" -j RETURN
          done
        fi

          # If an inbound packet belongs to an established socket, route it to the
          # loopback interface.
          iptables -t mangle -A ISTIO_INBOUND  -m socket -j ISTIO_DIVERT || echo "No socket match support"

          # Otherwise, it's a new connection. Redirect it using TPROXY.
          iptables -t mangle -A ISTIO_INBOUND  -j ISTIO_TPROXY

   else

        for port in ${INBOUND_PORTS_INCLUDE}; do
            iptables -t mangle -A ISTIO_INBOUND --dport "${port}" -m socket -j ISTIO_DIVERT || echo "No socket match support"
            iptables -t mangle -A ISTIO_INBOUND --dport "${port}" -j ISTIO_TPROXY
        done
   fi
}

function istio_iptable_output() {
    local OR=${OUTBOUND_IP_RANGES_INCLUDE:-10.15.0.0/16,10.16.0.0/16}
    IFS=,

    IPTCMD=${IPTCMD:-iptables -t nat }

    ${IPTCMD} -F ISTIO_OUTPUT

    iptables -t mangle -A DMESH_MANGLE_PRE --match mark --mark 12 -j RETURN
    iptables -t mangle -A DMESH_MANGLE_POST --match mark --mark 12 -j RETURN
    ${IPTCMD} -A ISTIO_OUTPUT --match mark --mark 12 -j RETURN

    # Apply port based exclusions. Must be applied before connections back to self
    # are redirected.
    if [ -n "${OUTBOUND_PORTS_EXCLUDE}" ]; then
      for port in ${OUTBOUND_PORTS_EXCLUDE}; do
        ${IPTCMD} -A ISTIO_OUTPUT --dport "${port}" -j RETURN
      done
    fi

    # 127.0.0.6 is bind connect from inbound passthrough cluster
    ${IPTCMD} -A ISTIO_OUTPUT -o lo -s 127.0.0.6/32 -j RETURN

    # Redirect app calls back to itself via Envoy when using the service VIP or endpoint
    # address, e.g. appN => Envoy (client) => Envoy (server) => appN.
    ${IPTCMD} -A ISTIO_OUTPUT -o lo ! -d 127.0.0.1/32 -j ISTIO_TPROXY

    ${IPTCMD} -A ISTIO_OUTPUT -m owner --uid-owner 0 -j RETURN
    for gid in ${PROXY_GID}; do
      # Avoid infinite loops. Don't redirect Envoy traffic directly back to
      # Envoy for non-loopback traffic.
      ${IPTCMD} -A ISTIO_OUTPUT -m owner --gid-owner "${gid}" -j RETURN
    done

    # Skip redirection for Envoy-aware applications and
    # container-to-container traffic both of which explicitly use
    # localhost.
    ${IPTCMD} -A ISTIO_OUTPUT -d 127.0.0.1/32 -j RETURN

    if "${OR}" == "*" ; then
        # Must be mangle table
        iptables -t mangle -A DMESH_MANGLE_OUT -p udp -j MARK --set-mark 1337
        iptables -t mangle -A DMESH_MANGLE_PRE -p udp -j ISTIO_TPROXY
    else
        for cidr in ${OR}; do
          iptables -t mangle -A DMESH_MANGLE_OUT -p udp -d ${cidr} -j MARK --set-mark 1337
          iptables -t mangle -A DMESH_MANGLE_PRE -p udp -d ${cidr} -j ISTIO_TPROXY
        done
    fi

    iptables -t mangle -A DMESH_MANGLE_PRE -j RETURN

    #${IPTCMD} -A ISTIO_OUTPUT  -j MARK --set-mark 1337

}


# Outbound capture, using iptables TPROXY.
function ipt_capture_udp() {

    #iptables -t mangle -A DMESH_MANGLE_PRE -d 127.0.0.1/32 -j RETURN


    # Outbound
    iptables -t mangle -A DMESH_MANGLE_PRE --match mark --mark 1337 -j ISTIO_TPROXY

}



