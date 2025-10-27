BASE:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
REPO?=$(shell basename $(BASE))

# Tools directory (this imported makefile, should be in tools/common.mk)
TOOLS:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

# Source dir (same as BASE and ROOT_DIR ?)
SRC_DIR:=$(shell dirname $(TOOLS))

-include ${HOME}/.local.mk
-include ${SRC_DIR}/.local.mk

BUILD_DIR?=/tmp
OUT?=${BUILD_DIR}/${REPO}

# Compiling with go build will link the local machine glibc
# Debian 11 is based on 2.31, testing is 2.36
GOSTATIC=CGO_ENABLED=0  GOOS=linux GOARCH=amd64 go build -ldflags '-s -w -extldflags "-static"'

# Requires docker login ghcr.io -u vi USERNAME -p TOKEN
GIT_REPO?=${REPO}

# Skaffold can pass this
# When running pods, label skaffold.dev/run-id is set and used for log watching
IMAGE_TAG?=latest
export IMAGE_TAG

# ghcr.io - easiest to login from github actions. Doesn't seem to work in cloudrun
#DOCKER_REPO?=ghcr.io/costinm/${GIT_REPO}
DOCKER_REPO?=costinm


BASE_DISTROLESS?=gcr.io/distroless/static
#BASE_IMAGE?=debian:testing-slim

# Does not include the TLS keys !
#BASE_IMAGE?=debian:testing-slim
# Alpine based, full of debug tools.
BASE_IMAGE?=nicolaka/netshoot

export PATH:=$(PATH):${HOME}/go/bin

echo:
	@echo BASE: ${BASE}
	@echo SRC_DIR: ${SRC_DIR}
	@echo OUT: ${OUT}
	@echo DOCKER_REPO: ${DOCKER_REPO}
	@echo BASE_IMAGE: ${BASE_IMAGE}
	@echo REPO: ${REPO}
	@echo MAKEFILE_LIST: $(MAKEFILE_LIST)
	@echo BIN: ${BIN}

	# From skaffold or default
	@echo IMAGE: ${IMAGE}
	@echo IMAGE_TAG: ${IMAGE_TAG}
	@echo IMAGE_REPO: ${IMAGE_REPO}
	@echo PUSH_IMAGE: ${PUSH_IMAGE}
	@echo BUILD_CONTEXT: ${BUILD_CONTEXT}


	# When running in a skafold environment
	# https://skaffold.dev/docs/builders/builder-types/custom/#contract-between-skaffold-and-custom-build-script
	# BUILD_CONTEXT=/x/sync/dmesh-src/ugate-ws/meshauth
    # IMAGE=ghcr.io/costinm/meshauth/meshauth-agent:0cc2116-dirty
    # PUSH_IMAGE=true
    # SKIP_TEST, PLATFORMS
    #
	# Not documented:
	#  IMAGE_TAG=0cc2116-dirty
    #  INVOCATION_ID=92f7287ba5a443f0872b11ace7c82ef2
    # SKAFFOLD_USER=intellij
    # SKAFFOLD_INTERACTIVE=false
    # LOGNAME=costin
    # IMAGE_REPO=ghcr.io/costinm/meshauth/meshauth-agent
	#
	#
    # When running in cluster, https://skaffold.dev/docs/builders/builder-types/custom/#custom-build-script-in-cluster
    # KUBECONTEXT
    # NAMESPACE
    #

# 1. Create a tar file with the desired files (BIN, PUSH_FILES)
# 2. Send it as DOCKER_REPO/BIN:latest - using BASE_IMAGE as base
# 3. Save the SHA-based result as IMG
# 4. Set /BIN as entrypoint and tag again
#
# Makefile magic: ":=" is evaluated once when the rule is read, so we can't use it here
# With "=" it's evaluate multiple times if used as in push3
# Turns out the simplest solution is to just use temp files.
.ONESHELL:
_push: BIN?=${REPO}
_push: IMAGE_REPO?=${DOCKER_REPO}/${BIN}
_push: IMAGE?=${IMAGE_REPO}:${IMAGE_TAG}
_push: echo
	@mkdir -p ${OUT}/etc/ssl/certs/
	@cp /etc/ssl/certs/ca-certificates.crt ${OUT}/etc/ssl/certs/
	# Push an image and save the tag in .image1.${BIN}
	cd ${OUT} && tar -cf - ${PUSH_FILES} usr/local/bin/${BIN} etc/ssl/certs | gcrane append -f - \
       -b ${BASE_IMAGE} \
       -t ${IMAGE} > ${OUT}/.image1.${BIN}

	@echo cat= $(shell cat ${OUT}/.image1.${BIN})
	cat ${OUT}/.image1.${BIN}

	gcrane mutate `cat ${OUT}/.image1.${BIN}` -t ${IMAGE} --entrypoint /usr/local/bin/${BIN} > ${OUT}/.image

#_push3: IMAGE?=${DOCKER_REPO}/${DOCKER_IMAGE}:${IMAGE_TAG}
#_push3: IMG1=$(shell cd ${OUT} && tar -cf - ${PUSH_FILES} usr/local/bin/${BIN} etc/ssl/certs | gcrane append -f - \
#       -b ${BASE_IMAGE} -t ${IMAGE} )
#_push3: IMG=$(shell gcrane mutate ${IMG1} -t ${IMAGE} --entrypoint /usr/local/bin/${BIN} )
#_push3:
#	@echo ${IMG} > ${OUT}/.image

#
#_push2: IMAGE?=${DOCKER_REPO}/${DOCKER_IMAGE}:${IMAGE_TAG}
#_push2:
#	echo ${IMAGE}
#	(export IMG=$(shell cd ${OUT} && \
#        tar -cf - ${PUSH_FILES} ${BIN} etc/ssl/certs | \
#    	   gcrane append -f - -b ${BASE_IMAGE} \
#					 		  -t ${IMAGE} \
#    					   ) && \
#    	gcrane mutate $${IMG} -t ${IMAGE} \
#    	  --entrypoint /usr/local/bin/${BIN} \
#    	)

# TODO: add labels like    	  -l org.opencontainers.image.source="https://github.com/costinm/${GIT_REPO}"

# To create a second image with a different base without uploading the tar again:
#	gcrane rebase --rebased ${DOCKER_REPO}/gate:latest \
#	   --original $${SSHDRAW} \
#	   --old_base ${BASE_DISTROLESS} \
#	   --new_base ${BASE_DEBUG} \

_oci_base:
	gcrane mutate ${OCI_BASE} -t ${DOCKER_REPO}/${BIN}:base --entrypoint /${BIN}

_oci_image:
	(cd ${OUT} && tar -cf - ${PUSH_FILES} ${BIN} | \
    	gcrane append -f - \
    				  -b  ${DOCKER_REPO}/${BIN}:base \
    				  -t ${DOCKER_REPO}/${BIN}:${IMAGE_TAG} )

_oci_local: build
	docker build -t costinm/hbone:${IMAGE_TAG} -f tools/Dockerfile ${OUT}/


deps:
	go install github.com/google/go-containerregistry/cmd/gcrane@latest
	go install github.com/googlecloudplatform/gcsfuse/v2@master

	go install github.com/google/ko@latest #
	# v0.14.1

	# oapi curl
	go install github.com/danielgtaylor/restish@latest

	# Oapi to proto
	go install github.com/googleapis/gnostic-grpc@latest
	go install github.com/google/gnostic@latest

	# can convert oapi to proto, generate oapi from code.
	go install cuelang.org/go/cmd/cue@latest

	# To inspect / call (separate server )
	go install github.com/swaggest/swgui/cmd/swgui@latest

	# 3.0, templates supported, etc
	# Issue: pointers by default.
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

	# Heavy - otel, middleware, static router
	# Optional[T]
	# streaming encoding support
	go get -d github.com/ogen-go/ogen
	go install github.com/danielgtaylor/restish@latest

# Build a command under cmd/BIN, placing it in $OUT/usr/local/bin/$BIN
#
# Also copies ssl certs
#
# Params:
# - BIN
#
# Expects go.mod in cmd/ or cmd/BIN directory.
_build: BIN?=${REPO}
_build: DIR?=cmd/${BIN}
_build:
	@mkdir -p ${OUT}/usr/local/bin
	@echo GOSTATIC: ${BASE}/${DIR} -> ${OUT}/usr/local/bin/${BIN}
	@mkdir -p ${OUT}/etc/ssl/certs/
	cp /etc/ssl/certs/ca-certificates.crt ${OUT}/etc/ssl/certs/
	cd cmd/${BIN} && ${GOSTATIC} -o ${OUT}/usr/local/bin/${BIN} .


_buildcgo: DIR?=cmd/${BIN}
_buildcgo:
	@echo CGO: ${BASE}/${DIR} -> ${OUT}/usr/local/bin/${BIN}
	mkdir -p ${OUT}/usr/local/bin
	cd ${DIR} &&  CC=musl-gcc go build -gcflags='all=-N -l'  --ldflags '-linkmode external -extldflags "-static"' \
	   -tags ${GO_TAGS} -o ${OUT}/usr/local/bin/${BIN} .
	#cd ${DIR} &&  go build -a -gcflags='all=-N -l' -ldflags '-extldflags "-static"' \
 	#   -tags ${GO_TAGS} -o ${OUT}/usr/local/bin/${BIN} .

_buildglibc: DIR?=cmd/${BIN}
_buildglibc:
	@echo CGO: ${BASE}/${DIR} -> ${OUT}/usr/local/bin/${BIN}
	mkdir -p ${OUT}/usr/local/bin
	cd ${DIR} &&  go build -a -gcflags='all=-N -l' -ldflags '-extldflags "-static"' \
 	   -tags ${GO_TAGS} -o ${OUT}/usr/local/bin/${BIN} .

# Other options:
# GOEXPERIMENT=boringcrypto and "-tags boringcrypto"


# https://honnef.co/articles/statically-compiled-go-programs-always-even-with-cgo-using-musl/
# apt install musl musl-tools musl-dev

# Sizes (example for ugate):
# static - 65M
# cgo - 103M (83M stripped)
# musl - 102M (82M stripped)
