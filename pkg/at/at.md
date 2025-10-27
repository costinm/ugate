# AT-Protocol and Mesh/Enterprise 

## TL;DR

- Good concepts:
    - FQDN per user and group (handle)
    - anchored on DNS and public TLS certs for bootstrap, like ACME
    - private key for identity
    - separate private key for signing
    - federation - using the private key to prove ownership
    - replication enabled by signing
    - mostly standard protocols
    - GIT-like - but not reusing GIT.
- The bad:
    - a lot of weird crypto - but not required, the P256 and JSON are supported in most cases
    - CBOR is not so common - but can be adapted to protobuf and JSON is supported

The core protocol can be 'bridged' to simpler and more standard representations, and 
it can be adapted to mesh/private use.

## Identity

Probably weakest part of the protocol, but still reasonably ok.

IMO the proper default for a server is:
- did:web for everything. 
- no DNS validation (too complex, DNS-SEC not required)
- bootstrap with the SSH public key (which can be generated and stored on client), perhaps a second recovery key
- the PDS private key for signing.
- p256 for everything, dump bitcoin legacy.
- dump the PLC DID - web is sufficient.
- standard OAuth2 - with aud the URL of the PDS.

Using the FQDN as external identity is a bit different from common use of 
user@domain. Effectively it's the same thing - you can mechanically convert, but
the protocol relies on the FQDN to exist and have a TLS cert. DNS is also supported,
but if you have the DNS - it also implies a real FQDN is allocated to each user.

This creates some pain/complexity - but the end result is very good if it can 
be extended, i.e. users get a real hostname, and the 'hosting' part can be 
extended to support more than just PDS.

The core is good: there are 2 private keys, one controls identity and the other for the content.
The 'identity' is expressed as an easily accessible JSON that declares the content signing key.

The 'public' identity is a FQDN and mutable, based on standard internet security.
If you control a domain - DNS or HTTPS - you have a 'handle', which can point to the keys ('DID').
Not clear how often handle is resolved/updated to DID - but it is just a reference to the key.


As with the mesh, there is a 'delegation' and trust model. 

It is ultimately anchored on public DNS and public cert providers (ACME),
but I think this is easy to fix/patch for mesh, using the mesh root of trust and internal DNS.

The key is stored in the global database and links back to handles.

Unfortunately DNS-SEC is not required - but (assuming a good implementation) respected.
ACME doesn't require DNS-SEC either - in both cases the use of DNS is 'on first use', 
and may perform verification from multiple points. 

## Event Streams

Plain binary websocket, one way (server->client)

'subscription' with 'message' types.

Event IDs and 'backfill window' similar to K8S - but IDs are related to the stream, not native.

DAG-CBOR encoding - can work with json as well. Header (1/-1 for message/error, t type) + body

## DID

DID (decentralized identifier) is a URI that can be decoupled from the DNS and TLS certs. It defines some restrictions on syntax, and a json 
document that is associated and verified based on the ID.

```json
{
  "@context": [
    "https://www.w3.org/ns/did/v1",
    "https://w3id.org/security/suites/ed25519-2020/v1"
  ]
  "id": "did:example:123456789abcdefghi",
  "authentication": [{
    
    "id": "did:example:123456789abcdefghi#keys-1",
    "type": "Ed25519VerificationKey2020",
    "controller": "did:example:123456789abcdefghi",
    "publicKeyMultibase": "zH3C2AVvLMv6gmMNam3uVAjZpfkcJCwDwnZn6z3wXmqPV"
  }]
}
```

The '@context' is the json XML namespace equivalent,
and is optional. It defines datetime as '2020-12-20T19:17:47Z' 

The ID allows .-_ and %xx - but can be restricted by method. 

The URL restricts ";", starts with "/", allows query and fragment. Some query params are reserved and used in the DID:

- 'service' - ascii string, must be in the doc 
- 'relativeRef' - resource relative to service
- versionId - of the doc, can be a hash.
- versionTime
- hl - hash link, of the did doc.

The DID 'resolves' to the document - but more important the document can be verified based on the DID.

There are 2 distinct operations: discovery (resolve) and verification, both defined in the DID method.

An important feature is that "DID controllers" can prove their identity to others using the doc.

Terms:
- DID doc - the json
- DID controller = who can modify/control the DID doc
- DID URL - the DID + path, which can be resolved
- DID - the URL without path
- DID subject - who is identified by the DID (user, pod, etc). Can be same as controller, or different.
- registry - the 'database' where DIDs are recorded and discovered



## PDS

PDS is the web server holding the data for one or many users. It has a protocol - but 
no reason other protocols can't be supported, or different storage mechanism.

Primary security is https - can be mapped to mesh security using the roots.

All records are signed - 

## K8S and Istio

- like Istio, it is based on 'identity'
- unlike Istio and K8S, content is signed. This is enables many scaling and security capabilities.


## Use as a '.internal' social/chat app

The mobile and web apps are OSS, so it should be possible to get them running in private mode.

## Formats/crypto

###  multiformat and 'commit'

ASN.1 Certificate is 'sequence', algorithm ID, bitstring, with serialNumber, issuer, subject, validity, subject public key, optional issuer/subject UIDs and extensions.

The 'DN' is a big mess with origins in the telegraph world, most of the internet is built on FQDNs, and so is atproto. Getting rid of 'email' and all other DN variants greatly simplify everything and allows mapping everything to DNS (and DNS-SEC).

# dasl.ing

Further restrictions on options:

- CIDv1 with base32 (b) encoding - DNS friendly
- raw (0x55) and dCBOR (0x71) content
- SHA256 - 0x12

One step further: on DNS representation skip the 'b\x55\x12', and just use base32(sha256). If the options are restricted to a canonical form - no need for the multiform.



### D-CBOR

Equivalent to DER vs BER. Great for signed structs because they can be round-tripped. Can still use full CBOR (or proto, etc) for blobs.


# Tools

https://pdsls.dev/at/did:plc:uuqeqhfxq2fccgf3d2kpbtor

https://blue.mackuba.eu/stats/ - 
https://bskycharts.edavis.dev/edavis.dev/bskycharts.edavis.dev/index.html


https://blueview.app/login - analytics ?

https://bsky.app/profile/costinm.bsky.social/rss

https://blueskyfeedcreator.com/pricing - feed generator

https://dev.uniresolver.io/ - not only bsky
- did:github, did:iid, did:dns, did:content, did:key, did:peer
- github: index.jsonld file in master of ghdid repo.

- https://blue.mackuba.eu/scanner/ - get labels for DID

- https://github.com/mary-ext/bluesky-labeler-scraping - list of labelers on github

- https://github.com/bluesky-social/jetstream - wss firehose. Docker image, filters, etc. 

# Var

- json-ld is XML with namespace - using @context, @id, 