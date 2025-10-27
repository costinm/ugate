
# Multi-framework configuration

All programs need config - and it is easy to get trapped into 'my needs are unique, 
all existing config method are flawed and some corner cases don't meet my needs', 
resulting in even more config methods.

A far from complete list:
- command line - with a couple of styles and countless libraries
- env variables
- Windows Registry and Mac plists (XML)
- custom 'domain specific languages' made for the app
- json, yaml, toml

Some are 'startup only', other support notifications and dynamic updates, other full 
config reload.

In recent years K8S and 'distributed' applications added the concept of having a 
'control plane' and CI/CD that not only stores all the config but can also distribute it to 'canaries' first, test the impact and grdually roll it out.

K8S is mixing many (good) use cases and features and tends to require
full adoption of the entire set, including specific ways to schedule workloads and 
different networking - but the *concepts* and reasons for making some choices are reusable.

# Reusing K8S config concepts

Each config object has a type (group + name), with an OpenAPI schema which can generate corresponding types in all languages.

Configs have metadata that tracks changes and allow efficient replication and update, and labels.

A relatively simple JSON API (OpenAPI compatible) HTTP service allows distribution of configs - but it is not strictly required. 



## Mapping Caddy

Caddy config is relatively close to K8S - each component uses a 'struct' that can 
be loaded from json, equivalent to K8S CRD. 

The 'module id' is equivalent to K8S 'Group + Version + Name' type, with the convention
that a specific 'namespace' (== Group) has a specific semantic. For example http.handlers is the group for all modules implementing Caddy version of http handler (not the standard one !). 

You could automatically create CRDs for each module, and load them at startup - the tricky part is that K8S uses references while Caddy config is in-line.

That means the config object for 'host modules' (that use plugins) needs to be adjusted to replace the in-line plugin with just the name of the config.




## Mapping spf13/Cobra

Typical 'main' uses a set of 'Commands' - which are similar to the K8S CRD, structs defining all CLI parameters. At startup the command line and env variables are mapped to the structure.

Given a config struct it should be possible to auto-generate the Cobra object - the viper package already has plugins allowing loading configs from K8S or other sources, but with a slightly different API and model.

## Mapping real K8S

## Mapping filesystem configs

- directory or single file
- yaml or other formats, with or without templates
- 

# Signing the configs

If the configs are fetched from a 'trusted source' - APIserver, git repo, https server - you can reasonably trust them and extra signatures are not adding much.

If the files are local (cached, pushed) - there is a small risk that they have been modified. Usually this also means the host is compromised - but it may be an escalation of privilege case where only the config storage or distribution are broken - we don't want this to spread.


# RPC, CLI and configs

A CLI command takes a couple of arguments, constructs a 'struct' and runs a function.

An RPC receives a struct and runs a function. The main differene is that it typically
needs to handle many concurrent requests, and it is 'long lived'.

Some modern CLI apps fork and leave a small server (SSH is a great example), so next
requests will be fast and it can keep connections alive - it becomes a RPC server, with each 'CLI command' serialized to a struct.

Abstractions are supposed to make it easy to understand and use concepts - but they can also obfuscate and create complexity - there is no value in treating a CLI or
 config very different from an RPC or http service.

A 'config' object is used in K8S in a REST API or as a 'RPC' - the same object (Gateway, HttpRoute) is viewed as a config file or as command line flags.

There are obvious differenes - CLI and env variables can't be changed, but are most convenient. Mapping CLI/Env to structs (spf13 viper is a good example) and using those as either config or RPC is a nice unification.






