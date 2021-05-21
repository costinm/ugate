#!/bin/bash

# Startup for the dev image
# This includes tools to help debug and even source code to make changes.
# It includes a sshd server, to allow access even without kube.

set -x

function start_ssh() {
  # Check if host keys are present, else create them
  # /etc dir may be RO, use var/run

  mkdir -p /run/ssh /run/sshd

  # TODO: custom call to get a cert for SSH.
  if ! test -f /run/ssh/ssh_host_rsa_key; then
      ssh-keygen -q -f /run/ssh/ssh_host_rsa_key -N '' -t rsa
  fi

  if ! test -f /run/ssh/ssh_host_ecdsa_key; then
      ssh-keygen -q -f /run/ssh/ssh_host_ecdsa_key -N '' -t ecdsa
  fi

#  if ! test -f /var/run/ssh/ssh_host_ed25519_key; then
#      ssh-keygen -q -f /var/run/ssh/ssh_host_ed25519_key -N '' -t ed25519
#  fi

  # TODO: support certificates for client auth
  echo ${SSH_AUTH} > /run/ssh/authorized_keys

  # Set correct right to ssh keys
  chown -R root:root /run/ssh /run/sshd
  chmod 0700 /run/ssh
  chmod 0600 /run/ssh/*

  chmod 700 /run/sshd

  echo "======== Starting SSHD with ${SSH_AUTH}"

  /usr/sbin/sshd
}

start_ssh

cd /
env
/ko-app/ugate

