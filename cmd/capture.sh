#!/usr/bin/env bash

PROXY_HOME=${PROXY_HOME:-/ws/istio-proxy}

function nat_setup() {
    groupadd -g 1337 istio
    mkdir /opt/dmesh
    mkdir /opt/dmesh/bin
    mkdir /var/dmesh
    chgrp istio /opt/dmesh
    chgrp istio /opt/dmesh/bin
    chgrp istio /var/dmesh
    chmod 775 /var/dmesh
}


# Fow for network packet to app:
# raw/pre -> conntrack -> mangle/pre -> nat/pre -> route -> mangle/in -> filter/in -> sock
#
# Flow from app to net;
# route -> raw/out -> conntrack -> mangle/out -> route -> nat/out -> filter/out -> mangle/post -> nat/post

# Local IPs: hostname -I


# Once, at boot time. Can be cleaned with dmeshclean
#
# Route table and fwmark 1338 redirects everything to 'dmesh1' device.
#
# To capture/redirect to dmesh:
# iptables -t mangle -A DMESH_MANGLE_OUT -j MARK [ANYTHING] --set-mark 1338
#
# Example:
# iptables -t mangle -A DMESH_MANGLE_OUT -j MARK -p tcp -m tcp --dport 5227 --set-mark 1338
function dmeshinit() {
    U=${GWUSER:-costin}    # istio-proxy

    # Run with 'sg'

    ip tuntap add dev dmesh1 mode tun user $U group 1337
    ip addr add 10.12.0.5 dev dmesh1

    # No IP6 address - confuses linux
    ip link set dmesh1 up

    # Accept anything from dmesh1
    iptables -t filter -N DMESH_FILTER_IN
    ip6tables -t filter -N DMESH_FILTER_IN
    iptables -t filter -A INPUT -j DMESH_FILTER_IN
    ip6tables -t filter -A INPUT -j DMESH_FILTER_IN
    #iptables -t filter -A DMESH_FILTER_IN -i dmesh1 -j LOG --log-prefix "dmesh1-f-in "
    iptables -t filter -A DMESH_FILTER_IN -i dmesh1 -j ACCEPT
    ip6tables -t filter -A DMESH_FILTER_IN -i dmesh1 -j ACCEPT

    # Mark packets from dmesh1
    iptables -t mangle -N DMESH_MANGLE_PRE
    ip6tables -t mangle -N DMESH_MANGLE_PRE
    iptables -t mangle -A PREROUTING -j DMESH_MANGLE_PRE
    ip6tables -t mangle -A PREROUTING -j DMESH_MANGLE_PRE
    iptables -t mangle -A DMESH_MANGLE_PRE -i dmesh1 -j MARK --set-mark 1337
    ip6tables -t mangle -A DMESH_MANGLE_PRE -i dmesh1 -j MARK --set-mark 1337

    iptables -t mangle -N DMESH
    ip6tables -t mangle -N DMESH

    # Output from apps: what is included and what is not.
    iptables -t mangle -N DMESH_MANGLE_OUT
    ip6tables -t mangle -N DMESH_MANGLE_OUT

    # Common/fixed configuration for mangle/OUTPUT
    # This applies to packets sent from local applications
    iptables -t mangle -N DMESH_MANGLE_OUT_START
    ip6tables -t mangle -N DMESH_MANGLE_OUT_START
    iptables -t mangle -A OUTPUT -j DMESH_MANGLE_OUT_START
    ip6tables -t mangle -A OUTPUT -j DMESH_MANGLE_OUT_START

    # Istio-proxy
    iptables -t mangle -A DMESH_MANGLE_OUT_START -m owner --gid-owner $U -j RETURN
    ip6tables -t mangle -A DMESH_MANGLE_OUT_START -m owner --gid-owner $U -j RETURN
    # Localhost
    iptables -t mangle -A DMESH_MANGLE_OUT_START -d 127.0.0.1/32 -j RETURN
    ip6tables -t mangle -A DMESH_MANGLE_OUT_START -d ::1/128 -j RETURN
    iptables -t mangle -A DMESH_MANGLE_OUT_START -j DMESH_MANGLE_OUT
    ip6tables -t mangle -A DMESH_MANGLE_OUT_START -j DMESH_MANGLE_OUT

    iptables -t mangle -A OUTPUT -j DMESH_MANGLE_OUT
    ip6tables -t mangle -A OUTPUT -j DMESH_MANGLE_OUT

    #
    echo 2 > /proc/sys/net/ipv4/conf/dmesh1/rp_filter
    sysctl -w net.ipv4.ip_forward=1

    #ip addr add 2001:470:1f04:429::3/128 dev dmesh1
    #ip route add 2001:470:1f04:429:80::0/65 dev dmesh1
    ip route add fd::/8 dev dmesh1
    ip route add 10.10.0.0/16 dev dmesh1
    ip route add 10.12.0.0/16 dev dmesh1

    # Anything marked with 1338 will be routed to dmesh1 interface
    ip rule add fwmark 1338 lookup 1338
    #ip route add ::/0      dev dmesh1 src 2001:470:1f04:429::3 table 1338
    ip route add ::/0      dev dmesh1  table 1338
    ip route add 0.0.0.0/0 dev dmesh1 src 10.12.0.5            table 1338

    # 1337 means deliver to local host
    ip rule add fwmark 1337 lookup 1337
    ip rule add iif dmesh1 lookup 1337
    ip route add local 0.0.0.0/0 dev lo table 1337
    #ip route add local ::/0 dev lo table 1337

    # All
    iptables -t mangle -A PREROUTING -i dmesh1 -j TPROXY --tproxy-mark 1337/0xffffffff --on-port 15006
    iptables -t mangle -A PREROUTING -o dmesh1 -j TPROXY --tproxy-mark 1337/0xffffffff --on-port 15006
}

function dmeshclean() {

    ip rule delete from all fwmark 1338 lookup 1338
    ip route flush table 1338
    ip rule delete from all fwmark 1337 lookup 1337
    ip route flush table 1337

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

    ip link set dmesh1 down

    ip tuntap del dev dmesh1 mode tun || true
}

# Gateway mode, running as istio-proxy
function dmeshbg() {
    cd $PROXY_HOME

    ./dmesh > ./proxy.log 2>&1 &
     echo $! > ./proxy.pid
}

# Gateway mode, running as istio-proxy
function dmeshbggw() {
    cd $PROXY_HOME
    export DMESH_IF=none
    ./dmesh
}

function dmeshgw() {
    cp /ws/dmesh/bin/dmesh ${PROXY_HOME}/dmesh
    chown istio-proxy ${PROXY_HOME}/dmesh
    su -s /bin/bash -c "/usr/local/bin/dmeshroot.sh runbggw" istio-proxy
}

function dmeshon() {
    cp /ws/dmesh/bin/dmesh ${PROXY_HOME}/dmesh
    chown istio-proxy ${PROXY_HOME}/dmesh
    su -s /bin/sh -c "/usr/local/bin/dmeshroot.sh runbg" istio-proxy
    capture
}

# This is the main function to capture outbound traffic, by marking it to 1338 which is routed to dmesh tun device.
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
}

function capture_in() {

    #iptables -t mangle -A DMESH_MANGLE_PRE -p udp --sport 5228 -j LOG --log-prefix "dmesh-mangle-pre "

    iptables -t mangle -A DMESH_MANGLE_PRE -i dmesh1 -j ACCEPT
    iptables -t mangle -A DMESH_MANGLE_PRE -i dmesh1 -j RETURN

    #iptables -t mangle -A DMESH_MANGLE_PRE --match mark --mark 12 -j RETURN

    #iptables -t mangle -A DMESH_MANGLE_PRE -p udp ! -i dmesh1 --match mark --mark 12 -j RETURN

    iptables -t mangle -A DMESH_MANGLE_PRE -j MARK --set-mark 1338
}

function dmeshoff() {
    ip6tables -t mangle -F DMESH_MANGLE_OUT
    iptables -t mangle -F DMESH_MANGLE_OUT
    kill -9 $(cat /ws/istio-proxy/proxy.pid)
}

case "$1" in
    "init" )
        dmeshinit ;;
    "start" )
        dmeshon ;;
    "capture" )
        capture ;;
    "run" )
        dmeshon ;;
    "gw" )
        dmeshgw ;;
    "runbg" )
        dmeshbg ;;
    "runbggw" )
        dmeshbggw ;;
   "dnson" )
        captureDNSRedir ;;

   "dnsoff" )
        captureDNSStop ;;

    "stop" )
        dmeshoff ;;
    "clean" )
        dmeshoff

        dmeshclean ;;

    * )
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
    ;;
esac
