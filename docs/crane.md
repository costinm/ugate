# Crane

Crane is a tool for directly interacting with a OCI registry.
Using 'gcrane' variant which has gcr.io extensions.

An OCI registry is a HTTP(S) server supporting GET, POST for SHA-based blobs and a small tag->manifest.
The manifest stores the SHAs of the config and a list of diff.tar.gzip files. Crane can push/pull blobs, and modify 
the manifest and config.

Install: `go install github.com/google/go-containerregistry/cmd/gcrane@latest`
Or: `docker run --rm gcr.io/go-containerregistry/gcrane`

Basic features:
- tag - operates directly on the remote, no need to pull/tag/push. Faster than copy.
- cp - copy from one repo to another, set tag as well. 
- delete
- mutate - change labels, annotations, entrypoint, cmd, env, user. Can also 'append' a tarball - but must be a real .tar, can't be stdin. 
- append - take base, layer (can be stdin). "--set-base-image-annotation" to include annotation about base for the new image.
- export - get a tar for the image
- flatten - single layer, combine all layers
- ls - list tags in repo

Mutate also takes a "-o" to output a tarball image, or "-t" to tag. If not specified, push to the original image manifest.

Low level:
- blob - read a single blob, using @sha256..., output a .tar.gz to stdout
- config - dump image config ( entrypoint, env, layers ? )
- manifest - shows the list of tar.gz layers. Can be downloaded with blob 
- digest - get image digest by tag
- pull - oci, legacy or tarball

Advanced:
- rebase - take last layers from one image, add them to a different image. Replaces old_base with new_base
- GGCR_EXPERIMENT_ESTARGZ=1 env variable

Library:
- Source, Sinks - remote, tarball, daemon, layout Image/Write
- can interact with Docker daemon
- Index: remote/layout/random
- Layer: remote, tarball

```shell

crane config gcr.io/istio-testing/proxyv2:1.12-dev-distroless |jq .
# shows 10 layers - first  distroless, second our additions, last 8 small parts of istio
 "config": {
    "User": "65532",
    "Env": [
      "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
      "SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt",
      "ISTIO_META_ISTIO_PROXY_SHA=istio-proxy:ff44db02db5a99a7f06c31441c3a5a0f7ce9e2b4",
      "ISTIO_META_ISTIO_VERSION=1.12-alpha.eae0ae5c1c492ce59ec56f73552a808b91687d58"
    ],
    "Entrypoint": [
      "/usr/local/bin/pilot-agent"
    ],
    "WorkingDir": "/",
    "OnBuild": null
  },


$ gcrane manifest --platform linux/amd64 gcr.io/istio-testing/proxyv2:1.12-dev-distroless |jq .

 https://gcr.io/v2/istio-testing/proxyv2/blobs/sha256:c5dc4f258debef99ad7e7690d50bd879f1193553e0d36747e9626cd7ac3265f8
 https://storage.googleapis.com/artifacts.istio-testing.appspot.com/containers/images/sha256:c5dc4f258debef99ad7e7690d50bd879f1193553e0d36747e9626cd7ac3265f8

$ gcrane  blob gcr.io/istio-testing@sha256:ed95b4ae780017a8aed1e302277312b9def69adfaf61f5fe86a3cc8a626b5b50 | tar tvfz -
- /var/lib/dpkg/tzdata, netbase, base
- tzdata: usr/share/zoneinfo /usr/sbin/tzconfig
- netbase: /etc/protocols,services,rpc,ethertypes
- /etc/passwd,group, nsswitch, 
- /etc/ssl/certs/ca-certificates.crt
- base: /etc/host.conf, 
- ./lib/x86_64-linux-gnu/libc-2.31.so

gcrane append -f <(cd ../out/cert-ssh/bin && tar -cf - sshd)  -t gcr.io/dmeshgate/ssh-signerd/sshd:latest

```
