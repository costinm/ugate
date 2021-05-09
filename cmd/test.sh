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
test_setup() {
  _do_start iperf3 ${TOP} iperf3 -s
  _do_start gate ${TOP}/cmd/ugate/testdata/gate ${TOP}/build/ugate
  _do_start alice ${TOP}/cmd/ugate/testdata/alice ${TOP}/build/ugate
  _do_start bob ${TOP}/cmd/ugate/testdata/bob ${TOP}/build/ugate
}

test_run() {
  # Direct access
  iperf3 -c localhost:5201
  # Via ugate, whitebox TCP capture
  iperf3 -c localhost:12111

  # Via routes
  iperf3 -c 10.13.0.1:12111
  iperf3 -c 10.15.0.1:12211
  iperf3 -c 10.17.0.1:15311
}

test_cleanup() {
  TUNDEV=0 sudo setup.sh clean
  TUNDEV=1 sudo setup.sh clean
}
