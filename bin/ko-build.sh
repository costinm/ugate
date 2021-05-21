#!/usr/bin/env bash

#set -e

if ! [ -x "$(command -v ko)" ]; then
    GO111MODULE=on go get github.com/google/ko@latest
fi

export KO_DOCKER_REPO=$(echo $IMAGE | cut -d: -f 1)
TAG=$(echo $IMAGE | cut -d: -f 2)


env

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

#cd ${HOME}/src/istio

export GGCR_EXPERIMENT_ESTARGZ=1

#output=$(ko publish ./pilot/cmd/pilot-discovery  -t $TAG  | tee)
# --insecure-registry
# -B - use the repo + last part of cmd
output=$(ko publish --bare ./cmd/ugate -t $TAG  | tee)

ref=$(echo $output | tail -n1)

# Doesn't work if image is not local (ko.local)
#docker tag $ref $IMAGE
#if $PUSH_IMAGE; then
#    docker push $IMAGE
#fi



# IMAGE: localhost:5001/wps:TAG

# -B - use last component of the name

T=$(echo $IMAGE | cut -d: -f 3)
TAG=${T:-latest}

echo TAG $TAG

output=$(ko publish ./cmd/wps --insecure-registry \
 -t $TAG --disable-optimizations -B | tee)


ref=$(echo $output | tail -n1)

# Doesn't work - image is not local
#docker tag $ref $IMAGE
#if $PUSH_IMAGE; then
#    docker push $IMAGE
#fi
