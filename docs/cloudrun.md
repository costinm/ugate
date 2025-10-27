# Cloudrun ssh

The original project was a hack to debug in cloudrun. This is still possible.

## Authentication

 gcloud run services add-iam-policy-binding [SERVICE_NAME] \
    --member="allUsers" \
    --role="roles/run.invoker"


bindings:
- members:
  - allUsers
  role: roles/run.invoker

policy:
  - role: roles/run.invoker
    members:
    - serviceAccount:...
    condition:
      expression: request.path == "/handleEvent"
  - role: roles/run.invoker
    members:
    - allUsers
    condition:
      expression: request.path == "/publicWebhookEndpoint"


gcloud run services set-iam-policy SERVICE policy.yaml

There are 2 ways to authenticate:

export TOKEN=$(gcloud auth print-identity-token --impersonate-service-account SERVICE_ACCOUNT_EMAIL --audiences='AUDIENCE')


gcloud run services add-iam-policy-binding SERVICE \
  --member='USER:EMAIL' \
  --role='roles/run.invoker'

gcloud run services add-iam-policy-binding RECEIVING_SERVICE \
  --member='serviceAccount:CALLING_SERVICE_IDENTITY' \
  --role='roles/run.invoker'

curl -H "Authorization: Bearer $(gcloud auth print-identity-token)" SERVICE_URL
or 
gcloud run services proxy SERVICE --project PROJECT-ID

Another way:
X-Serverless-Authorization: Bearer ID_TOKEN

curl "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/identity?audience=[AUDIENCE]" \
     -H "Metadata-Flavor: Google"

ID_TOKEN=$(curl -0 -X POST https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/SERVICE_ACCOUNT:generateIdToken \
  -H "Content-Type: text/json; charset=utf-8" \
  -H "Authorization: Bearer $STS_TOKEN" \
  -d @- <&ltEOF | jq -r .token
  {
      "audience": "SERVICE_URL"
  }
EOF
)
echo $ID_TOKEN



### Disable auth

 annotations:
      run.googleapis.com/invoker-iam-disabled: true
