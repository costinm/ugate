ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
OUT=${ROOT_DIR}/../out/ugate

#IMAGE ?= gcr.io/dmeshgate/ugate
REPO ?= ghcr.io/costinm/ugate
IMAGE ?= ${REPO}/ugate
KO_DOCKER_REPO ?= gcr.io/dmeshgate/ugate
#KO_DOCKER_REPO ?= costinm/ugate
export KO_DOCKER_REPO

all/cr: docker/dev push/dev push/ko  run/cloudrun

deploy: deploy/cloudrun deploy/helm

deploy/cloudrun: docker push/ugate run/cloudrun


deploy/helm: docker push/ugate run/helm

docker:
	docker build -t ${IMAGE}:latest .

docker/dev:
	docker build -t ${IMAGE}-dev:latest -f tools/dev/Dockerfile.devbase \
		tools/dev

docker/dev-istio:
	#docker pull gcr.io/istio-testing/build-tools:master-latest
	docker build -t ${IMAGE}-dev-istio:latest -f tools/dev/Dockerfile.istio-dev \
		tools/dev
	docker push ${IMAGE}-dev-istio:latest

push/dev:
	docker push ${IMAGE}-dev:latest

run/dev:
	 docker run -it --entrypoint /bin/bash gcr.io/dmeshgate/ugate-dev:latest

docker/devui:
	#docker pull golang:latest
	docker build -t ${IMAGE}-dev:cinamon-latest -f tools/dev/Dockerfile.cinamon tools/dev

push/devui:
	docker push ${IMAGE}-dev:cinamon-latest

run/devui:
	 docker run -it \
 		--entrypoint /bin/bash \
 		--rm --name dev \
             -p 18080:8080 -p 32000:22000 -p 8444:8444 \
             -v /x/sync/dmesh-src/ugate-ws:/work \
 		${IMAGE}-dev:cinamon-latest

run/docker-image:
	docker run -P -v /ws/dmesh-src/work/s1:/var/lib/istio \
		-v ${ROOT_DIR}:/ws \
		--name ugate \
		--cap-add NET_ADMIN \
		-p 443:9999 \
	   ${IMAGE}:latest

run/docker-test:
	docker stop ugate || true
	docker rm ugate || true
	docker run -P -v /ws/dmesh-src/work/s1:/var/lib/istio \
		-v ${ROOT_DIR}:/ws \
		--name ugate \
		--cap-add NET_ADMIN \
		-p 443:9999 \
	   ${IMAGE}:latest \
	   /ws/build/run.sh


push/docker.ugate: docker push/ugate

push/ugate:
	docker push ${IMAGE}:latest

cm-install:
	kubectl apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/v1.8.0/cert-manager.yaml
	#kubectl -n istio-system apply -f manifests/istio-issuer.yaml

deploy/kiali:
	helm install \
  --namespace istio-system \
  --set auth.strategy="anonymous" \
  --repo https://kiali.org/helm-charts \
  kiali-server \
  kiali-server

fw/kiali:
	kubectl port-forward svc/kiali 20001:20001 -n istio-system&

# Using Intellij plugin: missing manifest features
# Build with buildpack: 30 sec to deploy
# Build with docker: 26 sec
# Both use skaffold
# Faster than docker.
#push/ko:
#	(cd  cmd/ugate && ko publish . --bare)
#
#deps/ko:
#	go install github.com/google/ko@latest

# Run ugate in cloudrun.
# Storage: Env variables, GCP resources (buckets,secrets,k8s)
# Real cert, OIDC tokens via metadata server.
# https://ugate-yydsuf6tpq-uc.a.run.app
run/cloudrun: #push/docker.ugate
	gcloud beta run services replace manifests/knative-ugate.yaml --platform managed --project dmeshgate --region us-central1
	gcloud run services update-traffic ugate --to-latest --platform managed --project dmeshgate --region us-central1

run/cloudrun2: #push/docker.ugate
	gcloud beta run services replace manifests/knative-ugate.yaml --platform managed --project dmeshgate --region us-central1
	gcloud run services update-traffic ugate --to-latest --platform managed --project dmeshgate --region us-central1

run/cloudrun3:
	gcloud alpha run deploy  ugatevm --sandbox=minivm  --platform managed --project dmeshgate \
 		--region us-central1 --image gcr.io/dmeshgate/ugate:latest --command /usr/local/bin/run.sh --allow-unauthenticated --use-http2 --set-env-vars="SSH_AUTH=$(cat ~/.ssh/id_ecdsa.pub)" --use-http2

run/sshcr:
	 ssh -v  -o StrictHostKeyChecking=no -o ProxyCommand='hbone https://ugatevm-yydsuf6tpq-uc.a.run.app:443/dm/127.0.0.1:22'  \
 		root@ugate-yydsuf6tpq-uc.a.run.app:443

run/helm:
	helm upgrade --install --create-namespace ugate \
		--namespace ugate manifests/charts/ugate/

run/helm-istio-system:
	helm upgrade --install --create-namespace ugate-istio-system \
		--namespace istio-system manifests/charts/ugate/

test/run-iptables:
	docker run -P  \
		-v ${ROOT_DIR}:/ws \
		--rm \
		-w /ws \
		--entrypoint /bin/sh \
		--cap-add NET_ADMIN \
	   ${IMAGE}:latest \
	   -c "make test/iptables"


# Should be run in docker, as root
test/iptables:
	mkdir build
	./cmd/ugate/iptables.sh
	iptables-save |grep ISTIO > ${OUT}/iptables_def.out
	diff ${OUT}/iptables_def.out cmd/ugate/testdata/iptables/iptables_def.out

	INBOUND_PORTS_EXCLUDE=1000,1001 OUTBOUND_PORTS_EXCLUDE=2000,2001 ./cmd/ugate/iptables.sh
	iptables-save |grep ISTIO > ${OUT}/iptables_ex.out
	diff ${OUT}/iptables_ex.out cmd/ugate/testdata/iptables/iptables_ex.out

	IN=443 ./cmd/ugate/iptables.sh
	iptables-save |grep ISTIO > ${OUT}/iptables_443.out
	diff ${OUT}/iptables_443.out cmd/ugate/testdata/iptables/iptables_443.out

	IN=80,443 OUT=5201,5202 ./cmd/ugate/iptables.sh
	iptables-save |grep ISTIO > ${OUT}/iptables_443_5201.out
	diff ${OUT}/iptables_443_5201.out cmd/ugate/testdata/iptables/iptables_443_5201.out

HOSTS=c1 home

## For debug
run/home:
	HOST=ugate $(MAKE) remote/_run

run/c1:
	HOST=v $(MAKE) remote/_run

build:
	(cd ./cmd/ugate; CGO_ENABLED=0 go build -o ${OUT}/ugate .)

# Must have a $HOME/ugate dir
remote/_run: build
	ssh ${HOST} pkill ugate || true
	scp ${OUT}/ugate ${HOST}:/x/ugate
	ssh  ${HOST} "cd /x/ugate; HOME=/x/ugate /x/ugate/ugate"

update:
#	yq -j < cmd/ugate/testdata/ugate.yaml > cmd/ugate/testdata/ugate.json


deps:
	go install github.com/bufbuild/buf/cmd/buf@latest
	go install istio.io/tools/cmd/protoc-gen-docs@latest
	go install istio.io/tools/cmd/protoc-gen-crds@latest

	go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install github.com/bufbuild/connect-go/cmd/protoc-gen-connect-go@latest
	go install github.com/mikefarah/yq/v4@latest

	# debug tool for std grpc - need http/tcp equivalent
	go install -v github.com/grpc-ecosystem/grpcdebug@latest
	# Test tool
	go install github.com/bojand/ghz/cmd/ghz@latest
	curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl
	chmod +x ./kubectl
	mv ./kubectl /usr/local/bin

	go install github.com/knusbaum/go9p/cmd/mount9p@latest
	go install github.com/knusbaum/go9p/cmd/export9p@latest
	go install github.com/knusbaum/go9p/cmd/import9p@latest



proto-gen: PATH:=${HOME}/go/bin:${PATH}
proto-gen:
	cd proto && buf generate


# Other options:
# GOEXPERIMENT=boringcrypto and "-tags boringcrypto"
