# Example command to create a regular cluster.
function create_cluster() {

  cloud beta container --project "${PROJECT_ID}" clusters create \
    "${CLUSTER_NAME}" --zone "${CLUSTER_LOCATION}" \
    --no-enable-basic-auth \
    --cluster-version "1.20.8-gke.700" \
    --release-channel "rapid" \
    --machine-type "e2-standard-8" \
    --image-type "COS_CONTAINERD" \
    --disk-type "pd-standard" \
    --disk-size "100" \
    --metadata disable-legacy-endpoints=true \
    --scopes "https://www.googleapis.com/auth/devstorage.read_only","https://www.googleapis.com/auth/logging.write","https://www.googleapis.com/auth/monitoring","https://www.googleapis.com/auth/servicecontrol","https://www.googleapis.com/auth/service.management.readonly","https://www.googleapis.com/auth/trace.append" \
    --max-pods-per-node "110" \
    --num-nodes "1" \
    --enable-stackdriver-kubernetes \
    --enable-ip-alias \
    --network "projects/${PROJECT_ID}/global/networks/default" \
    --subnetwork "projects/${PROJECT_ID}/regions/${REGION}/subnetworks/default" \
    --no-enable-intra-node-visibility \
    --default-max-pods-per-node "110" \
    --enable-autoscaling \
    --min-nodes "0" \
    --max-nodes "9" \
    --enable-network-policy \
    --no-enable-master-authorized-networks \
    --addons HorizontalPodAutoscaling,HttpLoadBalancing,GcePersistentDiskCsiDriver \
    --enable-autoupgrade \
    --enable-autorepair \
    --max-surge-upgrade 1 \
    --max-unavailable-upgrade 0 \
    --workload-pool "${PROJECT_ID}.svc.id.goog" \
    --enable-shielded-nodes \
    --node-locations "${CLUSTER_LOCATION}"

}

# WIP: using an autopilot cluster for configurations. Note that only gateways can run right now inside the
# autopilot - other workloads should be in regular clusters (iptables)
function create_cluster_autopilot() {
  gcloud beta container --project "${PROJECT_ID}" clusters create-auto \
    "${CLUSTER_NAME}" --region "${CLUSTER_LOCATION}" \
    --release-channel "regular" \
    --network "projects/${PROJECT_ID}/global/networks/default" \
    --subnetwork "projects/${PROJECT_ID}/regions/${REGION}/subnetworks/default" \
    --cluster-ipv4-cidr "/17" \
    --services-ipv4-cidr "/22"

  gcloud container clusters get-credentials ${CLUSTER_NAME} --zone ${CLUSTER_LOCATION} --project ${PROJECT_ID}
  kubectl create ns istio-system

}

function setup_asm() {
  curl https://storage.googleapis.com/csm-artifacts/asm/install_asm_1.10 > install_asm
  chmod +x install_asm

  # Managed CP:
  ./install_asm --mode install --output_dir ${CLUSTER_NAME} --enable_all --managed
}

# Per project setup
function setup_project() {
  	gcloud services enable --project ${PROJECT_ID} vpcaccess.googleapis.com
  	gcloud compute networks vpc-access connectors create serverlesscon \
      --project ${PROJECT_ID} \
      --region ${REGION} \
      --subnet default \
      --subnet-project ${PROJECT_ID} \
      --min-instances 2 \
      --max-instances 10
}

