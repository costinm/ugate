# Load balancing and traffic shifting

UGate is NOT doing either. Use a K8S Gateway (envoy or anything else), no shortage of 
good implementations.

The goal is to provide connectivity:
1. Form endpoint to endpoint
2. From gateway to endpoints
3. From endpoint to gateway.

For HTTP it does need the hostname for routing. 

It does include a HTTP service, can serve some http files - and might do minimal route
on path prefix.

TODO: static config generator for envoy/nginx/caddy ?
TODO: test alongside gateways
