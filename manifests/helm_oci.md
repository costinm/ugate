# Publish Helm charts as OCI

https://helm.sh/docs/topics/registries/

- an OCI contains one or more helm repos, each with zero or more charts



# Use from terraform 

TODO

# Other discoveries

- https://zotregistry.dev/v2.0.3/
`helm repo add project-zot http://zotregistry.dev/helm-charts`
```shell
helm search repo project-zot

helm show all project-zot/zot
helm show values project-zot/zot


```

Tools:
- skopeo
- crane
- regclient
- oras