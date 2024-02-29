# Dev tools

Few tools that accelerate and simplify the development, and their impact on projects.

```shell
curl https://$NAME/ --connect-to $NAME:443:127.0.0.1:15007
```


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



## Okteto

Okteto is a remarcable developmet tool, with a model worth emulating in standalone charts to
build 'debuggable' images.

How it works:
- start with a base 'tools' image, including all the build tools - similar with the one used
to build istio.
- add a 'sshd' binary 
- add a 'syncthing' binary
- generate a sshd config - using the public key ~/.okteto/... as authorized

