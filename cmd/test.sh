#!/bin/bash

export TOP=$(cd .. && pwd)

export TUNUSER=${USER}

mkdir -p ${TOP}/build

_do_stop() {
  local name=shift
  kill -9 ${TOP}/build/${name}.pid
}

_do_start() {
  local name=shift
  local base=shift
  kill -9 ${TOP}/build/${name}.pid
  (cd $base && $* & )
  echo $! >${TOP}/build/${name}.pid
}

prepare_root() {
  sudo TUNDEV=0  ./setup.sh setup
  sudo TUNDEV=1  ./setup.sh setup
}

# setup test rig
# Alice: 6400, gvisor=(10.11.0.x, 10.10.0.x),
test_setup() {
  _do_start iperf3 ${TOP} iperf3 -s
  _do_start gate ${TOP}/cmd/ugate/testdata/gate ${TOP}/build/ugate
  _do_start alice ${TOP}/cmd/ugate/testdata/alice ${TOP}/build/ugate
  _do_start bob ${TOP}/cmd/ugate/testdata/bob ${TOP}/build/ugate
}

test_run() {
  echo Direct access - loopback iperf3 :5201
  iperf3 -c localhost:5201
  iperf3  -c 127.0.0.1 -p 5201 -u -b 4G
  iperf3  -c 10.1.10.228 -p 5201 -u -b 4G

  echo Alice:gvisor, tun network, listener by :port
  # Also: 10.10.0.2
  iperf3 -c 10.11.0.3 -p 6412

  echo Alice:gvisor-loopback - using the socket listener
  iperf3 -c 10.11.0.1 -p 6412

  echo Alice-QUIC-Gate-5201
  iperf3  -c 127.0.0.1 -p 5411
  iperf3  -c 127.0.0.1 -p 5411 -R

  echo Gate-RQUIC-Alice-5201
  iperf3  -c 127.0.0.1 -p 6013
  iperf3  -c 127.0.0.1 -p 6013 -R

  echo Bob-H2-Gate-5201
  iperf3  -c 127.0.0.1 -p 5111
  iperf3  -c 127.0.0.1 -p 5111 -R

  echo Gate-H2R-Bob-5201
  iperf3  -c 127.0.0.1 -p 6012
  iperf3  -c 127.0.0.1 -p 6012 -R

  #nuttcp -t -P 8888 -p 9999 -R 20G -u -i1s 127.0.0.1
  #nuttcp -S  -P 8888

  #iperf3  -c 10.1.10.228 -p 15101 -b 4G #-t 50000
  #
}

test_cleanup() {
  TUNDEV=0 sudo setup.sh clean
  TUNDEV=1 sudo setup.sh clean
}
