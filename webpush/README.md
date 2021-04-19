# Webpush Messaging 

## Goals

- support authenticated e2e encrypted messages, for control plane and data plane
- communication between the control plane components, including in-process and same node
- interop with Istio/XDS, CNCF, pubsub, etc

## API

- like CNCF, the 'core' is based on HTTP interface and Mux
- messages are http requests with encrypted body and JWT auth (VAPID)
- message 'envelope' - headers, url is not encrypted, treated as a request

## Receiving messages

- each client implements the regular push protocol, acting as server
- reverse H2 over H2, websocket or WebRTC can be used to send. 

## Topics

The topic is represented as a regular URL. Publishing is equivalent 
with posting to the URL. 

URL format: /msg/TOPIC
Subscribe (pull long lived connections): /sub/SUBID
Upstream brokers or push clients: list of URLs with same /msg/TOPIC

Unlike HTTP, multiple handlers can be registered on a topic.

