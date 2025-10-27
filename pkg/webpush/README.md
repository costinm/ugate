# Webpush Messaging 

## Goals

- support push and pubsub using authenticated, e2e encrypted messages
  - for data plane, abstracting other pubsub mechanisms or providing minimal storage-less delivery
  - for control plane, for distributing config without maintaining long-lived connections.
  - interop with Istio/XDS, CNCF, pubsub, etc

The main design idea is that in a 'mesh' or VPC, each workload (Pod, VM) that subscribes
to a topic is directly reachable. 

This is effectively the same as Webhooks - but using Webpush encryption and auth for
messages that cross multiple hops. 

A per-node agent - similar to Istio Ambient - can handle the crypto and security and
directly send messages to the pod, with network policy restricting the port.

## API

- like CNCF Eventing, Webhooks, CloudRun pubsub - the 'core' is based on HTTP interface.
- messages are http requests with encrypted body and JWT auth (VAPID)
- message 'envelope' - headers, url is not encrypted, treated as a request

## Receiving messages

- each subscriber (client) implements the regular push protocol, acting as Webpush server
- it is assumed that some infrastructure or transport can handle hanging GET equivalent,
  outside of the scope of this package. The requirement is that messages are injected as
  if the broker/infra made a HTTP call to the subscriber.

## Topics

The topic is represented as a regular URL. Publishing is equivalent 
with posting to the URL. 

URL format: /msg/TOPIC
Subscribe (pull long lived connections): /sub/SUBID
Upstream brokers or push clients: list of URLs with same /msg/TOPIC

Unlike HTTP, multiple handlers can be registered on a topic.

