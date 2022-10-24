# Configuration in the age of K8S and XDS

A config library should support a range of config mechanisms, with increased dependency list.

Requirements:
- minimal config using only env variables, friendly to containers
- flags and CLI should be used for client apps, not servers - but may be used optionally
- yaml config file -  reloaded on demand
- K8S ConfigMap, Secrets
- XDS configs
- http-based dynamic interface.

Env and flags are only set at startup, should provide defaults and bootstrap config.

# Others

- https://github.com/mwitkow/go-flagz, https://github.com/mwitkow/java-flagz - for java JMXBeans is used !

Supports etcd. 
Name, help, value, callbacks when changed dynamically.
Go version compatible with cobra.
Supports k8s ConfigMap watcher, prom metrics
/debug/flagz - introspection of runtime configuration, UI

Old - 6years

# JMX

JMX is java variant of SNMP - which is the 'original' telemetry and configuration 
interface. It is a push-friendly protocol.

