BIN=ugated

-include ${HOME}/.env.mk
include tools/common.mk

REGION?=us-central1

IMAGE ?= ${DOCKER_REPO}/${BIN}
WORKLOAD_NAMESPACE?=default


all: build/static build/cgo _push

ugated: build/static _push

build: build/static build/cgo

build/statuc: BIN=sshm
build/static: _build


push/ugated:
	$(MAKE) _push BIN=ugated

update:
	go get -u



#all/fortio: BIN=sshm
#all/fortio: DOCKER_IMAGE=fortio-sshm
#all/fortio:
#	#$(MAKE) _push BASE_IMAGE=fortio/fortio:latest BIN=sshc
#	docker build -t costinm/fortio-sshm:latest -f manifests/Dockerfile.fortio manifests/
#	docker push costinm/fortio-sshm:latest

build/cgo:
	$(MAKE) _buildcgo  DIR=./cmd/ugated GO_TAGS="lwip" BIN=ugated-cgo

run/docker-image:
	docker run -P -v /ws/dmesh-src/work/s1:/var/lib/istio \
		-v ${BASE}:/ws \
		--name ugate \
		--cap-add NET_ADMIN \
		-p 443:9999 \
	   ${IMAGE}:latest

run/docker-test:
	docker stop ugate || true
	docker rm ugate || true
	docker run -P -v /ws/dmesh-src/work/s1:/var/lib/istio \
		-v ${BASE}:/ws \
		--name ugate \
		--cap-add NET_ADMIN \
		-p 443:9999 \
	   ${IMAGE}:latest \
	   /ws/build/run.sh

# Run ugate in cloudrun.
# Storage: Env variables, GCP resources (buckets,secrets,k8s)
# Real cert, OIDC tokens via metadata server.
# https://ugate-yydsuf6tpq-uc.a.run.app
run/cloudrun: #push/docker.ugate
	gcloud beta run services replace manifests/knative-ugate.yaml --platform managed --project dmeshgate --region us-central1
	gcloud run services update-traffic ugate --to-latest --platform managed --project dmeshgate --region us-central1

run/cloudrun2: #push/docker.ugate
	gcloud run services replace manifests/knative-ugate.yaml --platform managed --project dmeshgate --region us-central1
	gcloud run services update-traffic ugate --to-latest --platform managed --project dmeshgate --region us-central1

run/cloudrun3:
	gcloud alpha run deploy  ugatevm --sandbox=minivm  --platform managed --project dmeshgate \
 		--region us-central1 --image gcr.io/dmeshgate/ugate:latest --command /usr/local/bin/run.sh --allow-unauthenticated --use-http2 --set-env-vars="SSH_AUTH=$(cat ~/.ssh/id_ecdsa.pub)" --use-http2

run/sshcr:
	 ssh -v  -o StrictHostKeyChecking=no -o ProxyCommand='hbone https://ugatevm-yydsuf6tpq-uc.a.run.app:443/dm/127.0.0.1:22'  \
 		root@ugate-yydsuf6tpq-uc.a.run.app:443

run/helm:
	helm upgrade --install --create-namespace ugate --namespace ugate manifests/charts/ugate/

test/run-iptables:
	docker run -P  \
		-v ${BASE}:/ws \
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

# Must have a $HOME/ugate dir
remote/_run: build
	ssh ${HOST} pkill ugate || true
	scp ${OUT}/ugate ${HOST}:/x/ugate
	ssh  ${HOST} "cd /x/ugate; HOME=/x/ugate /x/ugate/ugate"


# Replace the CR service
cr/replace:
	cat manifests/cloudrun.yaml | \
	DEPLOY="$(shell date +%H%M)" IMG="$(shell cat ${OUT}/.image)" envsubst | \
     gcloud alpha run --project ${PROJECT_ID} services replace -

cr/fortio:
	cat manifests/cloudrun-fortio.yaml | \
	DEPLOY="$(shell date +%H%M)" IMG="$(shell cat ${OUT}/.image)" envsubst | \
     gcloud alpha run --project ${PROJECT_ID} services replace -

proxy/fortio:
	gcloud run services proxy costin-fortio --region us-central1 --port 8082

crbindings: REGION=us-central1
crbindings:
	gcloud run services add-iam-policy-binding  --project ${PROJECT_ID} --region ${REGION} sshc  \
      --member="serviceAccount:k8s-default@${PROJECT_ID}.iam.gserviceaccount.com" \
      --role='roles/run.invoker'

crauth:
	gcloud run services add-iam-policy-binding  --project ${PROJECT_ID} --region ${REGION} sshc  \
      --member="user:${GCLOUD_USER}" \
      --role='roles/run.invoker'

crauth/all: REGION=us-central1
crauth/all:
	gcloud run services add-iam-policy-binding   --project ${PROJECT_ID} --region ${REGION} sshc  \
      --member="allUsers" \
      --role='roles/run.invoker'

iam-gcs:
	gcloud projects add-iam-policy-binding costin-asm1  --member serviceAccount:costin-asm1.svc.id.goog[dns-system/default] --role "roles/storage.objectUser"

# gcloud projects add-iam-policy-binding costin-asm1  --member group:costin-asm1.svc.id.goog:/allAuthenticatedUsers/ --role "roles/storage.objectUser"

    # group:costin-asm1.svc.id.goog:/allAuthenticatedUsers/

# SSH via a local jumphost
jssh:
	ssh -o StrictHostKeyChecking=no  -J localhost:15022 sshc.${SSHD} -v


# SSH to a CR service using a h2 tunnel.
# Works if sshd is handling the h2 port, may forward to the app.
# Useful if scaled to zero, doesn't require maintaining an open connection (but random clone)
cr/h2ssh: CR_URL?=$(shell gcloud run services --project ${PROJECT_ID} --region ${REGION} describe ${SERVICE} --format="value(status.address.url)")
cr/h2ssh:
	ssh -o ProxyCommand="${HOME}/go/bin/h2t ${CR_URL}_ssh/tun" \
        -o StrictHostKeyChecking=no \
        -o "SetEnv a=b" \
         sshc.${SSHD} -v

# Using the test certs:
#		-o "UserKnownHostsFile ssh/testdata/known-hosts" \
#		-i ssh/testdata/id_ecdsa \

ssh:
	 ssh -o StrictHostKeyChecking=no  -J localhost:2222 sshc.${SSHD} -v


ssh/keygen:
	rm -rf testdata/keygen
	mkdir -p testdata/keygen
	ssh-keygen -t ecdsa   -f testdata/keygen/id_ecdsa -N ""

ssh/getcert: CRT=$(shell cat testdata/keygen/id_ecdsa.pub)
ssh/getcert:
	echo {\"public\":\"${CRT}\"} | \
 		grpcurl -plaintext  -d @   [::1]:8080 ssh.SSHCertificateService/CreateCertificate | \
 		jq -r .user > testdata/keygen/id_ecdsa-cert.pub

	echo {\"public\":\"${CRT}\"} | \
 		grpcurl -plaintext  -d @   [::1]:8080 ssh.SSHCertificateService/CreateCertificate




gcp/setup:
	gcloud --project ${PROJECT_ID} iam service-accounts create k8s-${WORKLOAD_NAMESPACE} \
	  --display-name "Service account with access to ${WORKLOAD_NAMESPACE} k8s namespace" || true

	# Grant the GSA running the workload permission to connect to the config clusters in the config project.
	# Will use the 'SetQuotaProject' - otherwise the GKE API must be enabled in the workload project.
	gcloud --project ${CONFIG_PROJECT_ID} projects add-iam-policy-binding \
			${CONFIG_PROJECT_ID} \
			--member="serviceAccount:k8s-${WORKLOAD_NAMESPACE}@${PROJECT_ID}.iam.gserviceaccount.com" \
			--role="roles/container.clusterViewer"
	# This allows the GSA to use the GKE and other APIs in the 'config cluster' project.
	gcloud --project ${CONFIG_PROJECT_ID} projects add-iam-policy-binding \
			${CONFIG_PROJECT_ID} \
			--member="serviceAccount:k8s-${WORKLOAD_NAMESPACE}@${PROJECT_ID}.iam.gserviceaccount.com" \
			--role="roles/serviceusage.serviceUsageConsumer"

	# Also allow the use of TD
	gcloud projects add-iam-policy-binding ${PROJECT_ID} \
	  --member serviceAccount:k8s-${WORKLOAD_NAMESPACE}@${PROJECT_ID}.iam.gserviceaccount.com \
	   --role roles/trafficdirector.client

	gcloud secrets add-iam-policy-binding mesh \
        --member=serviceAccount:k8s-${WORKLOAD_NAMESPACE}@${PROJECT_ID}.iam.gserviceaccount.com \
        --role="roles/secretmanager.secretAccessor"

# 6 free versions, 10k ops
gcp/secret:
	gcloud secrets create mesh --replication-policy="automatic"
	gcloud secrets versions add mesh --data-file="/path/to/file.txt"

# Helper to create a secret for the debug endpoint.
init-keys:
	mkdir -p ${OUT}/ssh
	(cd ${OUT}/ssh; ssh-keygen -t ecdsa -f id_ecdsa -N "")
	cp ${HOME}/.ssh/id_ecdsa.pub ${OUT}/ssh/authorized_keys


k8s/secret: init-keys
	kubectl -n ${WORKLOAD_NAMESPACE} delete secret sshdebug || true
	kubectl -n ${WORKLOAD_NAMESPACE} create secret generic \
 		sshdebug \
 		--from-file=authorized_key=${OUT}/ssh/authorized_keys \
 		--from-file=cmd=cmd.json \
 		--from-file=ssd_config=sshd_config \
 		--from-file=id_ecdsa=${OUT}/ssh/id_ecdsa \
 		--from-file=id_ecdsa.pub=${OUT}/ssh/id_ecdsa.pub
	rm -rf ${OUT}/ssh


perf-test-setup:
    # Using goben instead of iperf3
	goben -defaultPort :5201 &

perf-test:
	# -passiveClient -passiveServer
	goben -hosts localhost:15201  -tls=false -totalDuration 3s

perf-test-setup-iperf:
    # Using goben instead of iperf3
	iperf3 -s -d &


wasm/meshauth:
	mkdir -p /tmp/tinygo
	docker run --rm -v $(shell pwd)/..:/src  -u $(shell id -u) \
      -v ${BUILD_DIR}/tinygo:/home/tinygo \
      -e HOME=/home/tinygo \
      -w /src/meshauth tinygo/tinygo:0.26.0 tinygo build -o /home/tinygo/wasm.wasm -target=wasm ./wasm/

