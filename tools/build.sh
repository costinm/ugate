
oaigen() {
  # OAI to grpc proto.
  gnostic --pb-out=. --grpc-out=static static/openapi.yaml


  cue import static/openapi.yaml

  cue def static/openapi.cue -o static/cue-openapi.yaml --out openapi+yaml

  # Out to cue.mod/gen/PACKAGE...
  cue get go ./cmd/ --local -v

  # Cloned googleapis
#  protoc static/openapi.proto --descriptor_set_out=static/oapi.gnostic --include_imports -I/x/sync/app/protoc/include/ -Istatic -I/x/r/oapi-ws/googleapis

  # cue - go to OAI
}

gen_oapi() {
  /
  oapi-codegen -config ./static/oaipd-cfg.yaml static/openapi.yaml
}




# Build, create a ko-based docker image and push
push() {
    KO_DOCKER_REPO=${IMAGE} ko build --tags ${VERSION} --bare \
      --image-label org.opencontainers.image.source="https://github.com/costinm/gcpdiag" \
      --image-label org.opencontainers.image.revision=$(git rev-parse HEAD) \
      --push=true github.com/costinm/gcpdiag/cmd/gcpdiagd

    # Could use ko on a local image and tag/push instead
    docker pull ${IMAGE}
}

# Run the image locally
local() {
  docker pull ${IMAGE}
  docker run -it --rm   -u "$(id -u):$(id -g)" \
     -p 8080:8080 \
     -e "USER=$(id -n -u)" \
       -e "GROUP=$(id -n -g)" \
       -e "HOME=$HOME" \
       -e "LANG=$LANG" \
       -e "SHELL=/bin/bash" \
       -v "/tmp:$PWD" \
       -v "$HOME/.config/gcloud:/home/.config/gcloud" \
       -w "$PWD" \
     ${IMAGE}:latest
}

tfd() {

  USE_TTY=""
  CWD=$(pwd)
  [ -t 0 ] && USE_TTY="-it"

  exec docker run $USE_TTY \
    --rm \
    -u "$(id -u):$(id -g)" \
    -e "USER=$(id -n -u)" \
    -e "GROUP=$(id -n -g)" \
    -e "HOME=$HOME" \
    -e "LANG=$LANG" \
    -e "SHELL=/bin/bash" \
    -v "$CWD:$CWD" \
    -v "$HOME/.config/gcloud:/home/.config/gcloud" \
    -w "$CWD" \
    mirror.gcr.io/hashicorp/terraform:light "$@"

}

deploy() {
#  for r in "" ; do
#      #
#      #
#      gcloud --project ${CONFIG_PROJECT_ID} projects add-iam-policy-binding \
#                ${CONFIG_PROJECT_ID} \
#                --member="serviceAccount:${GSA}@${PROJECT_ID}.iam.gserviceaccount.com" \
#                --role=${r}
#    done
#  gcloud iam service-accounts add-iam-policy-binding \
#     ${GSA}@${PROJECT_ID}.iam.gserviceaccount.com \
#    --member="user:costin@gmail.com " \
#    --role="roles/iam.serviceAccountTokenCreator"

  gcloud alpha run services --project ${PROJECT_ID} replace manifests/cloudrun.yaml
}

svcurl() {
  gcloud run services --project ${PROJECT_ID} --region ${REGION} describe ${SERVICE} --format="value(status.address.url)"
}


get() {
  curl -v -H"Authorization: Bearer $(shell gcloud auth print-identity-token)" ${SVC_URL}
}

r() {
  restish $SVC_URL
}

perms() {
  gcloud --project ${PROJECT_ID} iam service-accounts create ${GSA} \
        --display-name "Service account for investigations, read only" || true

  # For a set of rules, add the permission:
  #
  # Access to GKE
  for r in "roles/container.clusterViewer" "roles/container.clusterViewer" ; do
    #
    #
    gcloud --project ${CONFIG_PROJECT_ID} projects add-iam-policy-binding \
              ${CONFIG_PROJECT_ID} \
              --member="serviceAccount:${GSA}@${PROJECT_ID}.iam.gserviceaccount.com" \
              --role=${r}
  done


gcloud run services add-iam-policy-binding [SERVICE_NAME] \
    --member="allUsers" \
    --role="roles/run.invoker"

#  gcloud run services add-iam-policy-binding --region ${REGION} ${SERVICE}  \
#        --member="user:${GCLOUD_USER}" \
#        --role='roles/run.invoker'
#
#  gcloud projects add-iam-policy-binding ${PROJECT_ID}   \
#        --member="user:${GCLOUD_USER}" \
#        --role='roles/run.admin'


}


protoc_desc() {
  protoc static/openapi.proto --descriptor_set_out=static/oapi.gnostic.pb -I/x/sync/app/protoc/include/ -Istatic -I/x/r/oapi-ws/googleapis
}
