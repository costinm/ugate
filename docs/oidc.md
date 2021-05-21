# Support for OIDC auth

## Checking the tokens 

JWT OIDC tokens have a pretty clear format, using public key to verify.
The key used to verify is downloaded from a well-knwon location.

The main issue is how to map the OIDC token to a mesh identity.

For DMesh, the 'last resort' identity is the based on the SHA(public key) of the node.
This can be further mapped to a 'canonical service' or spiffee using a map.

If a OIDC identity exists, the following pattern can be used:

- trust domain is the OIDC issuer. If it is registered as an alias - it is trusted by 
default. Otherwise it must match an explicit Policy.
  
- the identified service account is mapped to a NAMESPACE. In practice, k8s namespace
is the primary unit of isolation. The service account will be 'default'. We can allow 
the workload to 'claim' a canonical name.
  
- if a certain naming pattern is used, like k8s-X-Y - we could also infer the service
account from the external name - in which case the self-claiming is not allowed.
  
## Generating tokens

In K8S we can exchange the audience-less token using the internal API, and implement STS.
STS appears supported by gRPC and others.

## Identities

Native identity depends on environment.

- K8S SA - system:serviceaccount:ugate:default

- workload identity - 'pool' set to PROJECT_ID.svc.id.goog. Google IAM uses 
    serviceAccount:PROJECT_ID.svc.id.goog[K8S_NAMESPACE/KSA_NAME]
  This is exposed using the metadata API - as well as via mounting. "requests in the first seconds may fail"
 ```shell
gcloud iam service-accounts add-iam-policy-binding \
  --role roles/iam.workloadIdentityUser \
  --member "serviceAccount:PROJECT_ID.svc.id.goog[K8S_NAMESPACE/KSA_NAME]" \
  GSA_NAME@PROJECT_ID.iam.gserviceaccount.com

kubectl annotate serviceaccount \
  --namespace K8S_NAMESPACE \
  KSA_NAME \
  iam.gke.io/gcp-service-account=GSA_NAME@PROJECT_ID.iam.gserviceaccount.com
```

- Metadata server:  http://metadata.google.internal (169.254.169.254:80)
```shell
curl -v  -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/identity?audience=foo.com

{"aud":"test.com","azp":"104797399114760319172",
"email":"584624515903-compute@developer.gserviceaccount.com", or "k8s--ugate--default@dmeshgate.iam.gserviceaccount.com"
"email_verified":true,
"exp":1621463417,"iat":1621459817,
"iss":"https://accounts.google.com",
"sub":"104797399114760319172"}
```  

