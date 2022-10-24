#!/usr/bin/env bash


# Route specific address or all to the TUN, for capture
# The dmesh app or user must be able to send packets !!!
#
# This only works well if there is a single default GW (home network).
# All traffic will be forced to go to the VPN/egress server, running a tproxy dmesh server.
# If the default GW is running a dmesh proxy - this is not needed.
#
# Inside a docker container this would also work -
function tunRoute() {

    # Find default GW
    # 'default via 192.168.0.254 dev wlp2s0 proto dhcp metric 600 '
    GW=$(/sbin/ip route | awk '/default/ { print $3 }')
    echo $GW > /tmp/DEFAULT_GW

    VPN=73.158.64.15

    GID=$(id -g costin)
    iptables -t nat -A DMESH_TUN_OUT -m owner --gid-owner ${GID} -j RETURN


    # Delete default route, add instead individual routes to egress(vpn) server, via normal GW
    ip route delete default
    ip route add $VPN/32 via $GW

    # Add a default route to dmesh1 device
    ip route add default dev dmesh1
}

function tunRouteOff() {
    GW=$(cat /tmp/DEFAULT_GW)

    ip route del default
    ip route add default via $GW
}


# Used for ip-level VPN. Creates linux-level NAT for 10.12.0.0/16 network on dmesh0
#
function ipt_fwd() {
    # Allow traffic initiated from VPN
    iptables -I FORWARD -i dmesh0 -m conntrack --ctstate NEW -j ACCEPT

    # Allow established traffic to pass back and forth
    iptables -I FORWARD -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT

    iptables -t nat  -A DMESH -s 10.12.0.0/16 -j MASQUERADE

    ip route add 10.12.0.0/16 dev dmesh1
}
