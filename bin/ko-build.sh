#!/usr/bin/env bash

# Script to use KO as a builder in Skaffold

# Skaffold passed IMAGE - including a SHA tag and repo
# - extract TAG and REPO
# - push the image
# - return the SHA of the image - will be used by skaffold

if ! [ -x "$(command -v ko)" ]; then
    GO111MODULE=on go get github.com/google/ko@latest
fi

export KO_DOCKER_REPO=$(echo $IMAGE | cut -d: -f 1)
TAG=$(echo $IMAGE | cut -d: -f 2)


#export GOMAXPROCS=1

# Default
#export KO_CONFIG_PATH=$BUILD_CONTEXT

export GOROOT=$(go env GOROOT)

# Set by skaffold
echo IMAGE=$IMAGE
echo KO_CONFIG_PATH=$KO_CONFIG_PATH
echo BUILD_CONTEXT=$BUILD_CONTEXT
echo KUBECONTEXT=$KUBECONTEXT
echo NAMESPACE=$NAMESPACE

export GGCR_EXPERIMENT_ESTARGZ=1

# --disable-optimizations
# --insecure-registry
# -B - use the repo + last part of cmd
output=$(ko publish --bare ./cmd/ugate -t $TAG  | tee)

ref=$(echo $output | tail -n1)
echo $ref
