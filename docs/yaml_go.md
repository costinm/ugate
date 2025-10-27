
# Yaml parsers in go

##  "gopkg.in/yaml.v3"

v3 last update: 2022 years ago

Garbage - bad errors, still refuses any compatibility with json tags,
not maintained. Do not use.

## "sigs.k8s.io/yaml"

Converts yaml to json, uses standard json package. 

No longer depends on go-yaml (has a fork), has deps on go-cmp and check.v1 for tests.

ghodss/yaml takes same approache - but doesn't seem maintained, better to use k8s.io.

## https://github.com/goccy/go-yaml

- supports yaml path
- used by etcd, tailscale, yq
