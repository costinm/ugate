function basic_cluster() {
 gcloud beta container --project "costin-asm1" clusters create "istio" --zone "us-central1-c" \
  --no-enable-basic-auth --cluster-version "1.25.1-gke.500" --release-channel "rapid" \
  --machine-type "e2-standard-4" --image-type "COS_CONTAINERD" --disk-type "pd-balanced" \
    --disk-size "100" --metadata disable-legacy-endpoints=true \
    --scopes "https://www.googleapis.com/auth/devstorage.read_only","https://www.googleapis.com/auth/logging.write","https://www.googleapis.com/auth/monitoring","https://www.googleapis.com/auth/servicecontrol","https://www.googleapis.com/auth/service.management.readonly","https://www.googleapis.com/auth/trace.append" \
    --max-pods-per-node "255" --spot --num-nodes "1" \
    --logging=SYSTEM,WORKLOAD --monitoring=SYSTEM --enable-ip-alias --network "projects/costin-asm1/global/networks/default" --subnetwork "projects/costin-asm1/regions/us-central1/subnetworks/default" \
    --no-enable-intra-node-visibility --enable-autoscaling --min-nodes "0" --max-nodes "4" --enable-dataplane-v2 --no-enable-master-authorized-networks \
    --addons HorizontalPodAutoscaling,HttpLoadBalancing,GcePersistentDiskCsiDriver --enable-autoupgrade --enable-autorepair --max-surge-upgrade 1 --max-unavailable-upgrade 0 \
    --enable-managed-prometheus --workload-pool "costin-asm1.svc.id.goog" --enable-shielded-nodes --no-shielded-integrity-monitoring --enable-image-streaming --node-locations "us-central1-c"
}

# Minimal
# 2 CPU one node
# For fleet enrolled: 722mCPU out of 940
# - event-exporter-gke 3
# - fluentbit-gke 100
# - gke-metrics-agent 11
# - mdp-controller 50 ???
# Without fleet:
# 558 out of 940
# -

function create_net6() {
  gcloud compute networks create ip6 --project=costin-istio3 --subnet-mode=custom --mtu=1460 --enable-ula-internal-ipv6 --bgp-routing-mode=regional

  gcloud compute networks subnets create NAME --project=costin-istio3 --range=IP_RANGE --stack-type=IPV4_ONLY --network=ip6 --region=REGION
}