

CA_DIR?=${HOME}/.secure/ca
mkdir -p ${CA_DIR}

# Should return all roots in the cluster, including from MeshConfig
kubectl get cm istio-ca-root-cert -o "jsonpath={.data['root-cert\.pem']}" >  ${CA_DIR}/root-cert.pem

kubectl get secret istio-ca-secret -n istio-system -o "jsonpath={.data['ca-cert\.pem']}" | base64 -d > ${CA_DIR}/ca-cert.pem
kubectl get secret istio-ca-secret -n istio-system -o "jsonpath={.data['ca-key\.pem']}" | base64 -d > ${CA_DIR}/ca-key.pem

# For 'plug-in' certificates:

# Use
# kubectl create secret generic cacerts -n istio-system \
  #      --from-file=cluster1/ca-cert.pem \
  #      --from-file=cluster1/ca-key.pem \
  #      --from-file=cluster1/root-cert.pem \
  #      --from-file=cluster1/cert-chain.pem

#kubectl get secret cacerts   -n  $(ISTIO_NAMESPACE) -o "jsonpath={.data['ca-cert\.pem']}" | base64 -d > ca-cert.pem
#kubectl get secret cacerts  -n  $(ISTIO_NAMESPACE) -o "jsonpath={.data['ca-key\.pem']}" | base64 -d > ca-key.pem
