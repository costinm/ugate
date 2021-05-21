# Dev tools

Few tools that accelerate and simplify the development, and their impact on projects.


## Ko

The ko-build.sh script will use 'go build' and push the result as a layer,
no docker involved. Best used with Skaffold.

Expected env variables:
- IMAGE - including tag. Set automatically by skaffold

Generally it works great if you have a base image with
all 'stable' components and you add a single go binary.
Ko can bundle files from .../kodata/ dir, and make it
available as $KO_DATA_PATH, including symlinks.
The example in readme is to include .git/HEAD

Ko creates reproductible builds - date is set.

For local repository or in-cluster with forwarded port -
it is faster, but requires keeping the port forwards open.

export KO_DOCKER_REPO=localhost:5000
ko.local == use local docker daemon
kind.local == use local kind

Can also cross-build, with "--platform=linux/amd64,linux/arm64"

# Skaffold 

It will also check if any file has changed, and in 'dev' mode
watch for changed files and re-deploy automatically.
It can also start a debugger and forward ports.

Without certificates and for faster time: deploy in-cluster registry, 
forward registry port to 5001. In-cluster registry runs on localhost:5001

- use shell script to build - most flexible, easy to plug in ko or other targets
- schema rewrite/refactoring removes comments.


```yaml
apiVersion: skaffold/v2beta14
kind: Config
metadata:
  name:    istiod
...


build:
  insecureRegistries:
    - localhost:5001

...

    # Building real istiod
    # Due to KO, which uses the image == last component of the go package
    - image: pilot-discovery
      context: /ws/istio-stable/src/istio.io/istio
      custom:
        buildCommand: /ws/dmesh-src/wpgate/bin/build-istiod.sh
        dependencies:
          paths:
            - "pilot/**"

      context: .
      custom:
        buildCommand: ../../bin/build-istiod.sh
        dependencies:
          paths:
            - "../../../istio/pilot/**"

deploy:
  kubectl:
    manifests:
      - ns.yaml
      - ../helm/istiod/charts/*.yaml

```

Env variables in the build command:

```shell
PUSH_IMAGE=true
IMAGE=gcr.io/dmeshgate/istiod:t-2021-05-15_11-23
IMAGE_REPO=gcr.io/dmeshgate/istiod
IMAGE_TAG=t-2021-05-15_11-23
SKAFFOLD_UPDATE_CHECK=false

```


## Okteto

Okteto is a remarcable developmet tool, with a model worth emulating in standalone charts to
build 'debuggable' images.

How it works:
- start with a base 'tools' image, including all the build tools - similar with the one used
to build istio.
- add a 'sshd' binary 
- add a 'syncthing' binary
- generate a sshd config - using the public key ~/.okteto/... as authorized

