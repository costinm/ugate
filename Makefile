OUT=build

#include ${HOME}/.hosts.mk

run/home:
	HOST=ugate $(MAKE) run

run/c1:
	HOST=v $(MAKE) run

# Must have a $HOME/ugate dir
run:
	CGO_ENABLED=0 go build -o ${OUT}/ugate ./cmd/ugate
	ssh ${HOST} pkill ugate || true
	scp ${OUT}/ugate ${HOST}:/x/ugate
	ssh  ${HOST} "cd /x/ugate; /x/ugate/ugate"

update:
	yq -j < cmd/ugate/testdata/ugate.yaml > cmd/ugate/testdata/ugate.json

IMAGE ?= gcr.io/dmeshgate/ugate

docker:
	docker build -t ${IMAGE}:latest .

run/docker-image:
	docker run -P -v /ws/dmesh-src/work/s1:/var/lib/istio \
		-v ${PWD}:/ws \
		--name ugate \
		--cap-add NET_ADMIN \
		-p 443:9999 \
	   ${IMAGE}:latest

run/docker-test:
	docker run -P -v /ws/dmesh-src/work/s1:/var/lib/istio \
		-v ${PWD}:/ws \
		--name ugate \
		--cap-add NET_ADMIN \
		-p 443:9999 \
	   ${IMAGE}:latest \
	   /ws/build/run.sh

push/docker.ugate: docker push/ugate

push/ugate:
	docker push ${IMAGE}:latest

# Using Intellij plugin: missing manifest features
# Build with buildpack: 30 sec to deploy
# Build with docker: 26 sec
# Both use skaffold

# Run ugate in cloudrun.
# Storage: Env variables, GCP resources (buckets,secrets,k8s)
# Real cert, OIDC tokens via metadata server.
cr/run: #push/docker.ugate
	gcloud beta run services replace k8s/cr.yaml --platform managed --project dmeshgate --region us-central1
	gcloud run services update-traffic ugate --to-latest --platform managed --project dmeshgate --region us-central1

gcp/secret:
	# projects/584624515903/secrets/ugate-key
	gcloud secrets create <SECRET-NAME> \
		--data-file <PATH-TO-SECRET-FILE> \
		--replication-policy user-managed \
		--project dmeshgate \
		--format json \
		--quiet

# Should be run in docker, as root
test/iptables:
	./cmd/ugate/iptables.sh
	iptables-save |grep ISTIO > build/iptables_def.out
	diff build/iptables_def.out cmd/ugate/testdata/iptables/iptables_def.out

	INBOUND_PORTS_EXCLUDE=1000,1001 OUTBOUND_PORTS_EXCLUDE=2000,2001 ./cmd/ugate/iptables.sh
	iptables-save |grep ISTIO > build/iptables_ex.out
	diff build/iptables_ex.out cmd/ugate/testdata/iptables/iptables_ex.out

	IN=443 ./cmd/ugate/iptables.sh
	iptables-save |grep ISTIO > build/iptables_443.out
	diff build/iptables_443.out cmd/ugate/testdata/iptables/iptables_443.out

	IN=80,443 OUT=5201,5202 ./cmd/ugate/iptables.sh
	iptables-save |grep ISTIO > build/iptables_443_5201.out
	diff build/iptables_443_5201.out cmd/ugate/testdata/iptables/iptables_443_5201.out
