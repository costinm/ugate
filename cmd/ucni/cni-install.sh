#!/bin/sh

# This script is used to install the CNI plugins on the host.
# It is intended to be simple and clean - a distroless installer that
# does the same thing is also possible, but less readable and riskier as
# it tends to become complex.

# Based on https://github.com/istio/cni/blob/release-1.1/deployments/kubernetes/Dockerfile.install-cni

# Expects to run in a container with the CNI binaries in /opt/cni/bin
# Base container can be alpine.


# Script to install Istio CNI on a Kubernetes host.
# - Expects the host CNI binary path to be mounted at /host/opt/cni/bin.
# - Expects the host CNI network config path to be mounted at /host/etc/cni/net.d.
# - Expects the desired CNI config in the CNI_NETWORK_CONFIG env variable.

# Ensure all variables are defined, and that the script fails when an error is hit.
set -u -e

# Helper function for raising errors
# Usage:
# some_command || exit_with_error "some_command_failed: maybe try..."
exit_with_error(){
  echo $1
  exit 1
}

function rm_bin_files() {
  echo "Removing existing binaries"
  rm -f /host/opt/cni/bin/cni-agent
}

# find_cni_conf_file
#   Finds the CNI config file in the mounted CNI config dir.
#   - Follows the same semantics as kubelet
#     https://github.com/kubernetes/kubernetes/blob/954996e231074dc7429f7be1256a579bedd8344c/pkg/kubelet/dockershim/network/cni/cni.go#L144-L184
#
function find_cni_conf_file() {
    cni_cfg=
    for cfgf in $(ls ${MOUNTED_CNI_NET_DIR}); do
        if [ "${cfgf: -5}" == ".conf" ]; then
            # check if it's a valid CNI .conf file
            type=$(cat ${MOUNTED_CNI_NET_DIR}/${cfgf} | jq 'has("type")' 2>/dev/null)
            if [[ "${type}" == "true" ]]; then
                cni_cfg=${cfgf}
                break
            fi
        elif [ "${cfgf: -9}" == ".conflist" ]; then
            # Check that the file is json and has top level "name" and "plugins" keys
            # NOTE: "cniVersion" key is not required by libcni (kubelet) to be present
            name=$(cat ${MOUNTED_CNI_NET_DIR}/${cfgf} | jq 'has("name")' 2>/dev/null)
            plugins=$(cat ${MOUNTED_CNI_NET_DIR}/${cfgf} | jq 'has("plugins")' 2>/dev/null)
            if [[ "${name}" == "true" && "${plugins}" == "true" ]]; then
                cni_cfg=${cfgf}
                break
            fi
        fi
    done
    echo "$cni_cfg"
}

function check_install() {
  cfgfile_nm=$(find_cni_conf_file)
  if [[ "${cfgfile_nm}" != "${CNI_CONF_NAME}" ]]; then
    if [[ "${CNI_CONF_NAME_OVERRIDE}" != "" ]]; then
       # Install was run with overridden cni config file so don't error out on the preempt check.
       # Likely the only use for this is testing this script.
       echo "WARNING: Configured CNI config file \"${MOUNTED_CNI_NET_DIR}/${CNI_CONF_NAME}\" preempted by \"$cfgfile_nm\"."
    else
       echo "ERROR: CNI config file \"${MOUNTED_CNI_NET_DIR}/${CNI_CONF_NAME}\" preempted by \"$cfgfile_nm\"."
       exit 1
    fi
  fi
  if [ -e "${MOUNTED_CNI_NET_DIR}/${CNI_CONF_NAME}" ]; then
    istiocni_conf=$(cat ${MOUNTED_CNI_NET_DIR}/${CNI_CONF_NAME} | jq '.plugins[]? | select(.type == "istio-cni")')
    if [[ "$istiocni_conf" == "" ]]; then
      echo "ERROR: istio-cni CNI config removed from file: \"${MOUNTED_CNI_NET_DIR}/${CNI_CONF_NAME}\""
      exit 1
    fi
  else
    echo "ERROR: CNI config file \"${MOUNTED_CNI_NET_DIR}/${CNI_CONF_NAME}\" removed."
    exit 1
  fi
}

function cleanup() {
  echo "Cleaning up and exiting."

  if [ -e "${MOUNTED_CNI_NET_DIR}/${CNI_CONF_NAME}" ]; then
    echo "Removing istio-cni config from CNI chain config in ${MOUNTED_CNI_NET_DIR}/${CNI_CONF_NAME}"
    CNI_CONF_DATA=$(cat ${MOUNTED_CNI_NET_DIR}/${CNI_CONF_NAME} | jq 'del( .plugins[]? | select(.type == "istio-cni"))')
    echo "${CNI_CONF_DATA}" > ${MOUNTED_CNI_NET_DIR}/${CNI_CONF_NAME}
  fi
  if [ -e "${MOUNTED_CNI_NET_DIR}/${KUBECFG_FILE_NAME}" ]; then
    echo "Removing istio-cni kubeconfig file: ${MOUNTED_CNI_NET_DIR}/${KUBECFG_FILE_NAME}"
    rm ${MOUNTED_CNI_NET_DIR}/${KUBECFG_FILE_NAME}
  fi
  rm_bin_files
  echo "Exiting."
}

# The directory on the host where CNI networks are installed. Defaults to
# /etc/cni/net.d, but can be overridden by setting CNI_NET_DIR.  This is used
# for populating absolute paths in the CNI network config to assets
# which are installed in the CNI network config directory.
HOST_CNI_NET_DIR=${CNI_NET_DIR:-/etc/cni/net.d}
MOUNTED_CNI_NET_DIR=${MOUNTED_CNI_NET_DIR:-/host/etc/cni/net.d}

CNI_CONF_NAME_OVERRIDE=${CNI_CONF_NAME:-}

# default to first file in `ls` output
# if dir is empty, default to a filename that is not likely to be lexicographically first in the dir
CNI_CONF_NAME=${CNI_CONF_NAME:-$(find_cni_conf_file)}
CNI_CONF_NAME=${CNI_CONF_NAME:-YYY-istio-cni.conflist}
KUBECFG_FILE_NAME=${KUBECFG_FILE_NAME:-ZZZ-istio-cni-kubeconfig}
CFGCHECK_INTERVAL=${CFGCHECK_INTERVAL:-1}


trap cleanup EXIT

# Clean up any existiang binaries / config / assets.
rm_bin_files

# Choose which default cni binaries should be copied
SKIP_CNI_BINARIES=${SKIP_CNI_BINARIES:-""}
SKIP_CNI_BINARIES=",$SKIP_CNI_BINARIES,"
UPDATE_CNI_BINARIES=${UPDATE_CNI_BINARIES:-"true"}

# Place the new binaries if the directory is writeable.
for dir in /host/opt/cni/bin /host/secondary-bin-dir
do
  if [ ! -w "$dir" ];
  then
    echo "$dir is non-writeable, skipping"
    continue
  fi
  for path in /opt/cni/bin/*;
  do
    filename="$(basename $path)"
    tmp=",$filename,"
    if [ "${SKIP_CNI_BINARIES#*$tmp}" != "$SKIP_CNI_BINARIES" ];
    then
      echo "$filename is in SKIP_CNI_BINARIES, skipping"
      continue
    fi
    if [ "${UPDATE_CNI_BINARIES}" != "true" -a -f $dir/$filename ];
    then
      echo "$dir/$filename is already here and UPDATE_CNI_BINARIES isn't true, skipping"
      continue
    fi
    cp $path $dir/ || exit_with_error "Failed to copy $path to $dir. This may be caused by selinux configuration on the host, or something else."
  done

  echo "Wrote Istio CNI binaries to $dir"
  #echo "CNI plugin version: $($dir/istio-cni -v)"
done

TMP_CONF='/istio-cni.conf.tmp'
# If specified, overwrite the network configuration file.
: ${CNI_NETWORK_CONFIG_FILE:=}
: ${CNI_NETWORK_CONFIG:=}
if [ -e "${CNI_NETWORK_CONFIG_FILE}" ]; then
  echo "Using CNI config template from ${CNI_NETWORK_CONFIG_FILE}."
  cp "${CNI_NETWORK_CONFIG_FILE}" "${TMP_CONF}"
elif [ "${CNI_NETWORK_CONFIG}" != "" ]; then
  echo "Using CNI config template from CNI_NETWORK_CONFIG environment variable."
  cat >$TMP_CONF <<EOF
${CNI_NETWORK_CONFIG}
EOF
fi

