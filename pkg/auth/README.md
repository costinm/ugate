# Auth package 

Features:
- dependency free implementation of Kubeconfig
- bootstrap using platform data - workload certs, JWT tokens, etc.
- support Webpush encryption and decryption
- basic deps-free OIDC parsing for JWT tokens
- webpush VAPID JWT support and subscriptions


# Dependencies

- ConfStore abstracts loading/saving of the objects used for auth - file, remote call, k8s, etc
