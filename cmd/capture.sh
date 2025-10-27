#!/usr/bin/env bash

PROXY_HOME=${PROXY_HOME:-/ws/istio-proxy}

HTTP_PORT=14080
U=${GWUSER:-build}    # istio-proxy
PROXY_GID="1337"


# Flow for network packet to app:
# raw/pre -> conntrack -> mangle/pre -> nat/pre -> route -> mangle/in -> filter/in -> sock
#
# Flow from app to net;
# route -> raw/out -> conntrack -> mangle/out -> route -> nat/out -> filter/out -> mangle/post -> nat/post
# Local IPs: hostname -I

# Naming:
# DMESH_${TABLE}_${HOOK}_${SUFFIX}
# The hook is pre, post, out
#


# For port 80 - we don't need tproxy or redirect - DNAT works too !
function http() {
    # From: https://v2.gost.run/en/redirect/, etc - no need of tproxy or original dst for HTTP or for SNI routing
    # This can also be sent directly to another host (TODO: check on pods)
    iptables -t nat -A OUTPUT -p tcp --dport 82 \
      -j DNAT \
      --to-destination 127.0.0.1:${HTTP_PORT}

       #--match multiport ! --dports 15001,1080 \

}

function init_tun() {
  local D=${1:-0}
  local T=dmesh${D}
  local TIP=10.16.${D}.1

  ip tuntap add dev $T mode tun user $U group 1337
  # No IP6 address - confuses linux
  ip addr add $TIP/24 dev $T
  #ip route add fd:8::0/64 dev $T
  ip link set $T up

  echo 2 > /proc/sys/net/ipv4/conf/$T/rp_filter

  #iptables -t filter -A DMESH_FILTER_IN -i dmesh1 -j LOG --log-prefix "dmesh1-f-in "
  #iptables -t filter -A DMESH_FILTER_IN -i $T -j ACCEPT
  #ip6tables -t filter -A DMESH_FILTER_IN -i $T -j ACCEPT

  # Mark packets
  #iptables -t mangle -A DMESH_MANGLE_PRE -i $T -j MARK --set-mark 1337
  #ip6tables -t mangle -A DMESH_MANGLE_PRE -i $T -j MARK --set-mark 1337

  # VIPs (services) and mesh IPs assigned to nodes are routed to the tun
  # This is the easy part - there is no real destination, 'real' hosts require
  # special processing if the router runs in same namespace, since it needs to
  # send packets to the real route/interface.

  #ip addr add 2001:470:1f04:429::3/128 dev dmesh1
  #ip route add 2001:470:1f04:429:80::0/65 dev dmesh1
  #ip route add fd:0${D}::/16 dev dmesh1
  ip route add 10.1.${D}.0/24 dev $T

  # Anything marked with 1001 will be routed to dmesh1 interface
  # This is in addition to all VIPs and other ranges with normal routes.
  #
  ip rule add fwmark 100${D} lookup 100${D}
  #ip route add ::/0      dev dmesh1 src 2001:470:1f04:429::3 table 1338
  #ip route add ::/0      dev $T  table 100${D}
  #ip route add 0.0.0.0/0 dev $T  table 100${D}
  ip route add local 0.0.0.0/0 dev lo  table 100${D}

  # Can't use -o with PREROUTING and tproxu - just i
  iptables -t mangle -A PREROUTING -i $T -p tcp \
      -j TPROXY --tproxy-mark 100${D} --on-port 14006
  iptables -t mangle -A PREROUTING -i $T -p udp \
      -j TPROXY --tproxy-mark 100${D} --on-port 14006

}

function init_veth() {
 local MARK=6

 ip netns add mesh

 ip link add dev veth-mesh type veth peer name veth-hostmesh
 ip link set veth-mesh netns mesh

 ip -n mesh addr add 10.253.2.1/24 dev veth-mesh
 ip netns exec mesh route add default dev veth-mesh

 ip addr add 10.253.1.1/24 dev veth-hostmesh

 # VIPs (services and istio virtual pod IPs) - standard route is sufficient
 ip route add 10.253.4.0/24 via 10.253.2.1 dev veth-hostmesh

 ip netns exec mesh ip addr
 ip -n mesh link set dev veth-mesh up
 ip -n mesh link set dev lo up
 ip link set dev veth-hostmesh up

 ip -n mesh rule add fwmark $MARK lookup $MARK
 ip -n mesh route add local 0.0.0.0/0 dev lo  table $MARK

 # Not clear if required
# ip netns exec mesh  iptables -t mangle -N DIVERT
# ip netns exec mesh  iptables -t mangle -A PREROUTING -p tcp -m socket -j DIVERT
# ip netns exec mesh  iptables -t mangle -A DIVERT -j MARK --set-mark $MARK
# ip netns exec mesh  iptables -t mangle -A DIVERT -j ACCEPT

 ip netns exec mesh iptables -t mangle -A PREROUTING -i veth-mesh -p tcp \
      -j TPROXY --tproxy-mark $MARK --on-port 14006
 # Ugate must run in this namespace - or pass a socket there

}

function del_veth() {
  ip link del dev vm1

  # https://gist.github.com/NiceRath/900f115f216c942283584c41baeb209f
  # Delete all
  nft flush ruleset
  nft list ruleset
}

function del_tun() {
  local D=${1:-0}
  local T=dmesh${D}

  iptables -t mangle -D PREROUTING -i $T -p tcp \
      -j TPROXY --tproxy-mark 100${D} --on-port 14006
  iptables -t mangle -D PREROUTING -i $T -p udp \
      -j TPROXY --tproxy-mark 100${D} --on-port 14006

  ip rule delete from all fwmark 100${D} lookup 100${D}
  ip route flush table 100${D}

  ip link set $T down
  ip tuntap del dev $T mode tun || true
}

# Once, at boot time. Can be cleaned with dmeshclean
#
# Route table and fwmark 1338 redirects everything to 'dmesh1' device.
#
# To capture/redirect to dmesh:
# iptables -t mangle -A DMESH_MANGLE_OUT -j MARK [ANYTHING] --set-mark 1338
#
# Example:
# iptables -t mangle -A DMESH_MANGLE_OUT -j MARK -p tcp -m tcp --dport 5227 --set-mark 1338
function onBoot() {
    #
    sysctl -w net.ipv4.ip_forward=1


   # Create the jump tables - for easy cleanup and org
    iptables -t filter -N DMESH_FILTER_IN
    ip6tables -t filter -N DMESH_FILTER_IN
    iptables -t mangle -N DMESH_MANGLE_PRE
    ip6tables -t mangle -N DMESH_MANGLE_PRE
    iptables -t mangle -N DMESH
    ip6tables -t mangle -N DMESH
    # Output from apps: what is included and what is not.
    iptables -t mangle -N DMESH_MANGLE_OUT
    ip6tables -t mangle -N DMESH_MANGLE_OUT
    # Common/fixed configuration for mangle/OUTPUT
    # This applies to packets sent from local applications
    iptables -t mangle -N DMESH_MANGLE_OUT_START
    ip6tables -t mangle -N DMESH_MANGLE_OUT_START

   # Hook the tables at the appropriate points
    iptables -t mangle -A PREROUTING -j DMESH_MANGLE_PRE
    ip6tables -t mangle -A PREROUTING -j DMESH_MANGLE_PRE
    iptables -t filter -A INPUT -j DMESH_FILTER_IN
    ip6tables -t filter -A INPUT -j DMESH_FILTER_IN
    iptables -t mangle -A OUTPUT -j DMESH_MANGLE_OUT_START
    ip6tables -t mangle -A OUTPUT -j DMESH_MANGLE_OUT_START

    # All
    # Istio also has ISTIO_OUTPUT on nat

    #init_tun 1
    init_tun 0


    # Istio-proxy
    iptables -t mangle -A DMESH_MANGLE_OUT_START -m owner --gid-owner $U -j RETURN
    ip6tables -t mangle -A DMESH_MANGLE_OUT_START -m owner --gid-owner $U -j RETURN

    # Localhost
    iptables -t mangle -A DMESH_MANGLE_OUT_START -d 127.0.0.1/32 -j RETURN
    ip6tables -t mangle -A DMESH_MANGLE_OUT_START -d ::1/128 -j RETURN

    iptables -t mangle -A DMESH_MANGLE_OUT_START -j DMESH_MANGLE_OUT
    ip6tables -t mangle -A DMESH_MANGLE_OUT_START -j DMESH_MANGLE_OUT




    # 1337 means deliver to local host - this is used with TPROXY
    # Requires root and transparent.
    ip rule add fwmark 1337 lookup 1337
    ip rule add iif dmesh1 lookup 1337

    ip route add local 0.0.0.0/0 dev lo table 1337
    #ip route add local ::/0 dev lo table 1337

  # Istio uses:
  #    iptables -t mangle -A ISTIO_TPROXY ! -d 127.0.0.1/32 -p tcp -j TPROXY --tproxy-mark 1337/0xffffffff --on-port 15006

  # Optimization for established:
  #     iptables -t mangle -A ISTIO_INBOUND -p tcp -m socket -j ISTIO_DIVERT || echo "No socket match support"
}

function debug() {
      ip rule list
      ip -s -d -a route list table all | grep -v "table local" |grep -v "table main"

}

function dmeshclean() {

    ip rule delete from all fwmark 1338 lookup 1338
    ip route flush table 1338
    ip rule delete from all fwmark 1337 lookup 1337
    ip route flush table 1337

    # Must first -F (flush), then delete the rule using (-D) then delete the chain (-X)

    iptables -t mangle -F DMESH_MANGLE_OUT
    iptables -t mangle -F DMESH_MANGLE_OUT_START
    iptables -t mangle -F DMESH
    iptables -t mangle -F DMESH_MANGLE_PRE
    iptables -t filter -F DMESH_FILTER_IN
    iptables -t filter -D INPUT -j DMESH_FILTER_IN
    iptables -t mangle -D PREROUTING -j DMESH_MANGLE_PRE
    iptables -t mangle -D OUTPUT -j DMESH_MANGLE_OUT
    iptables -t filter -X DMESH_FILTER_IN
    iptables -t mangle -X DMESH_MANGLE_PRE
    iptables -t mangle -X DMESH_MANGLE_OUT
    iptables -t mangle -X DMESH_MANGLE_OUT_START
    iptables -t mangle -X DMESH

    ip6tables -t mangle -F DMESH_MANGLE_OUT
    ip6tables -t mangle -F DMESH
    ip6tables -t mangle -F DMESH_MANGLE_PRE
    ip6tables -t filter -F DMESH_FILTER_IN
    ip6tables -t filter -D INPUT -j DMESH_FILTER_IN
    ip6tables -t mangle -D PREROUTING -j DMESH_MANGLE_PRE
    ip6tables -t mangle -D OUTPUT -j DMESH_MANGLE_OUT
    ip6tables -t filter -X DMESH_FILTER_IN
    ip6tables -t mangle -X DMESH_MANGLE_PRE
    ip6tables -t mangle -X DMESH_MANGLE_OUT
    ip6tables -t mangle -X DMESH
}



# This is the main function to capture outbound traffic, by marking it to 1338 which is routed
# to dmesh tun device.
function capture() {
    ip6tables -t mangle -F DMESH_MANGLE_OUT
    iptables -t mangle -F DMESH_MANGLE_OUT


    # Use destination (egress) as an exclude list.
    #
    # Gateway, via HE IP6. Used if we have IPv6
    ip6tables -t mangle -A DMESH_MANGLE_OUT -d 2001:470:1f04:429::2/128 -j RETURN


    # Gateway
    # h.webinf
    iptables -t mangle -A DMESH_MANGLE_OUT -d 73.158.64.15/32 -j RETURN
    iptables -t mangle -A DMESH_MANGLE_OUT -d 149.28.196.14/32 -j RETURN
    iptables -t mangle -A DMESH_MANGLE_OUT -d 10.1.10.0/24 -j RETURN

    # c1
    iptables -t mangle -A DMESH_MANGLE_OUT -d 104.196.253.32/32 -j RETURN


    # TODO: move capture to DMESH table.
    # Capture everything else
    iptables -t mangle -A DMESH_MANGLE_OUT -j MARK --set-mark 1338
    ip6tables -t mangle -A DMESH_MANGLE_OUT -j MARK --set-mark 1338
    iptables -t mangle -A DMESH_MANGLE_OUT -j ACCEPT
    ip6tables -t mangle -A DMESH_MANGLE_OUT -j ACCEPT
}

function ipt() {
  local A=$*

  iptables $A
  ip6tables $A
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


function captureOptIn() {
  local PORTS=$1
    ip6tables -t mangle -F DMESH_MANGLE_OUT
    iptables -t mangle -F DMESH_MANGLE_OUT

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

    iptables -t mangle -A DMESH_MANGLE_OUT -p tcp --dport 80 \
     --set-mark 1338

}

function capture_in() {

    #iptables -t mangle -A DMESH_MANGLE_PRE -p udp --sport 5228 -j LOG --log-prefix "dmesh-mangle-pre "

    iptables -t mangle -A DMESH_MANGLE_PRE -i dmesh1 -j ACCEPT
    iptables -t mangle -A DMESH_MANGLE_PRE -i dmesh1 -j RETURN

    #iptables -t mangle -A DMESH_MANGLE_PRE --match mark --mark 12 -j RETURN

    #iptables -t mangle -A DMESH_MANGLE_PRE -p udp ! -i dmesh1 --match mark --mark 12 -j RETURN

    iptables -t mangle -A DMESH_MANGLE_PRE -j MARK --set-mark 1338
}


#    "init" )
#        dmeshinit ;;
#    "start" )
#        dmeshon ;;
#    "capture" )
#        capture ;;
#    "run" )
#        dmeshon ;;
#    "gw" )
#        dmeshgw ;;
#    "runbg" )
#        dmeshbg ;;
#    "runbggw" )
#        dmeshbggw ;;
#   "dnson" )
#        captureDNSRedir ;;
#
#   "dnsoff" )
#        captureDNSStop ;;
#
#    "stop" )
#        dmeshoff ;;
#    "clean" )
#        dmeshoff
#
#        dmeshclean ;;

function help() {
  echo "'init' must be called once, followed by start/stop "
          echo
          echo "start: start proxy, running as istio-proxy/1337"
          echo "stop: stop proxy and interception"
          echo
          echo "gw: run as istio-proxy, gatway mode"
          echo
          echo "dnson/dnsoff: enable dns capture"
          echo
          echo "init: prepare"
          echo "clean: remove routes, devices and iptables"
}



CMD=$1
shift
$CMD $*
