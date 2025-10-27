
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

function dmeshoff() {
    ip6tables -t mangle -F DMESH_MANGLE_OUT
    iptables -t mangle -F DMESH_MANGLE_OUT
    kill -9 $(cat /ws/istio-proxy/proxy.pid)
}

# Setup the dirs and user/group.
function setup_dirs() {
    groupadd -g 1337 istio

    mkdir /opt/dmesh
    mkdir /opt/dmesh/bin
    mkdir /var/dmesh

    chgrp istio /opt/dmesh
    chgrp istio /opt/dmesh/bin
    chgrp istio /var/dmesh

    chmod 775 /var/dmesh
}

k8s_swaggger() {
  # URL
  #

  docker run \
      --rm \
      -p 80:8080 \
      -e URL=file:///k8s-swagger.json \
      -v $(pwd)/k8s-swagger.json:/k8s-swagger.json \
        swaggerapi/swagger-ui
}

gencue() {
  cue def $1_go_gen.cue -o openapi+yaml:api.$1.yaml
}


CMD=$1
shift
$CMD $*
