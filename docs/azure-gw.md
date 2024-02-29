
# Azure Gateway notes

https://learn.microsoft.com/en-us/azure/application-gateway/for-containers/overview

- "Application Gateway for Containers" - using Azure Resource Manager
- user deploys apps using Azure RM, uses Gateway API to configure, with "ALB" controller as CP.
- VNet for networking
- "User assigned managed identity"

Features:
- splitting, mTLS, routing APIs
- "near real time" updates on add/move pod, routes, probes
- elastic ingress to AKS
- *outside of AKS cluster and responsible for ingress*
- L7 - based on hostname, headers, etc
- mTLS to backend ( not on frontend !)
- also support BYO - a Frontend resource should be provisioned first.
- or fully managed bu ALB

Price: 
- 1 CU = 2500 persistent connections, 2Mbps throughput, 1 compute unit
- a compute unit is 50 connection/sec with TLS, 10 RPS (!)
- one instance == 10 CU, equivalent to a pod, mapped to to 10 CU
- old LB - 0.24/gateway-hour, 0.008 per capacity-unit-h, more for firewall
- example: 8 instances
  - 40 CU to handle 88Mbps
  - $179 fixed plus 467 variable = 646
  - 80 CU == 800 RPS, 200k persistent connections.
  - 0 min scale - $323 / month


Setup:
- create network in resource group: `az network alb create`
- create frontend `az network alb frontend create`
- delegate subnet - association resource: `az network vnet subnet update --delegations "trafficControlers"`
- delegate permissions `az role assignment create ...`
- create association - `az network alb association create ...`

Notes:
- status address type IPAddress by value is a FQDN !
- 