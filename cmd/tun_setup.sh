#!/bin/sh

# Setup the TUN device for capture.
#
# Must be run as root.
#
# Will setup rules for tagging using 1${N}0 1${N}1

export TUNUSER=${TUNUSER:-istio-proxy}
export N=${N:-0}

# Create a TUN device.
setupTUN() {
  echo Create dmesh${N}. owned by ${TUNUSER}
  echo Address: TUN=10.11.${N}.1 GVISOR=10.11.${N}.2
  echo 10.10.${N}.0/24 will be routed to dmesh${N}

  ip tuntap add dev dmesh${N} mode tun user ${TUNUSER} group ${TUNUSER}
  ip addr add 10.11.${N}.1/24 dev dmesh${N}
  # IP6 address may confuse linux
  ip -6 addr add fd::1:${N}/64 dev dmesh${N}

  ip link set dmesh${N} up

  # Route various ranges to dmesh1 - the gate can't initiate its own
  # connections to those ranges. Service VIPs can also use this simpler model.
  # ip route add fd::/8 dev ${N}
  ip route add 10.10.${N}.0/24 dev dmesh${N}


  # Don't remember why this was required
  echo 2 > /proc/sys/net/ipv4/conf/dmesh${N}/rp_filter
  sysctl -w net.ipv4.ip_forward=1
}

cleanup() {
  # App must be stopped
  ip tuntap del dev dmesh${N} mode tun

  ip rule delete  fwmark 1{N}1 priority 10  lookup 1{N}1
  ip route del default dev dmesh${N} table 1{N}1

  ip rule del fwmark 1{N}0 lookup 1{N}0
  ip rule del iif dmesh${N} lookup 1{N}0
  ip route del local 0.0.0.0/0 dev lo table 1{N}0
}


# Setup custom rules for egress capture using TUN.
# Mesh, DNS and VIP addresses should be routed to TUN directly, if
# DNS provides right modifications this is not needed.
#
# This is critical for UDP, where iptables doesn't work.
# iptables interception for TCP is still faster.
#
# - add a routing table (1338) to dmesh
# - all packets with mark 1338 will use the new routing table
# - route 10.10.0.0/16 via the tun
setup() {
  # For iptables capture/marks:
  # 101 means capture and send to TUN
  ip route add default dev dmesh${N} table 1${N}1
  ip rule add  fwmark 1${N}1 priority 10  lookup 1${N}1


  # 100 means deliver to local host
  ip route add local 0.0.0.0/0 dev lo table 1${N}0
  ip rule add fwmark 1${N}0 lookup 1${N}0
  # Anything from the TUN will be sent to localhost
  # That means packets injected into TUN.
  ip rule add iif dmesh${N} lookup 1${N}0
  #ip route add local ::/0 dev lo table ${N}0
}



stop() {
  iptables -t mangle -D OUTPUT -j DMESH_MANGLE_OUT${N}
  iptables -t mangle -D PREROUTING  -i dmesh${N} -j MARK --set-mark 1{N}0
  #iptables -t mangle -D PREROUTING -j DMESH_MANGLE_PRE

  iptables -t mangle -F DMESH_MANGLE_OUT${N} 2>/dev/null
  iptables -t mangle -X DMESH_MANGLE_OUT${N} 2>/dev/null
}

# Setup will create route-based rules for the NAT.
# This function intercepts additional packets, using
# Istio-style rules.
start() {
  GID=$(id -g ${TUNUSER})

  # -j MARK only works in mangle table !
  # Allows selecting a different route table
  # This is for preroute, i.e. incoming packets on an interface

  # Mark packets injected into dmesh1 so they get injected into localhost
  #iptables -t mangle -A DMESH_MANGLE_PRE -j MARK -p tcp --dport 5201 --set-mark 1338
  iptables -t mangle -A PREROUTING -i dmesh${N} -j MARK --set-mark 1{N}0

  # Capture outbound packets
  iptables -t mangle -N DMESH_MANGLE_OUT${N}
  iptables -t mangle -F DMESH_MANGLE_OUT${N}
  iptables -t mangle -A DMESH_MANGLE_OUT${N} -m owner --gid-owner "${GID}" -j RETURN

  # Capture everything else
  #iptables -t mangle -A DMESH_MANGLE_OUT -j MARK --set-mark 1338

  # Explicit
  #iptables -t mangle -A DMESH_MANGLE_OUT -p tcp -d 169.254.169.254 -j DROP

  # Explicit by-port capture, for testing
  #  iptables -t mangle -A DMESH_MANGLE_OUT -j MARK -p tcp --dport 5201 --set-mark 1{N}1
  iptables -t mangle -A DMESH_MANGLE_OUT${N} -j MARK -p udp --dport 12311 --set-mark 1{N}1

  #iptables -t mangle -A DMESH_MANGLE_OUT -j MARK -p tcp --dport 80 --set-mark 1338

  # Jump to the ISTIO_OUTPUT chain from OUTPUT chain for all tcp traffic.
  iptables -t mangle -A OUTPUT -j DMESH_MANGLE_OUT${N}
}

if [ "$1" = "init" ] ; then
  setupTUN
elif [ "$1" = "setup" ] ; then
  setup
elif [ "$1" = "start" ] ; then
  start
elif [  "$1" = "stop" ] ; then
  stop
elif [ "$1" = "clean" ] ; then
  cleanup
  stop
fi
