# Bsky ATproto - the good parts

My perspective on the ATproto - focusing on what I consider the core
protocol and the good elements. No protocol is perfect, and success is based 
on a few core decisions and luck, which seems to be on the side of Bsky.
Not sure if I'll publish the other side, it is a bit longer but not
so important. 

## Background

ATproto is intended for:
- sharing public info - like NNTP, RSS and countless others
- provide a vendor-neutral identity
- allow building a 'trust' model for users/organizations
- good security based on keypairs and signatures, but transparent/hidden for non-technical people

It is paired with Web and mobile applications covering the core
features and without a lot of bloat - and free hosting (not clear
what are the limits), which is important for adoption
by non-technical users. There is also good marketing, and goals 
that are attractive to both technical and non-technical users.

Critical: some element of luck and right timing - which is really the 
key to any protocol's success.

Bsky provides 'free' hosting under bsky.social - without a strong lock-in - 
so there are paths for revenue using hosting or additional features 
or pay-for-storage or support/management for other companies hosting 
their own - along with risks that larger players (or many smaller ones)
will overwhelm them, but this is the price for building trust and larger 
adoption.


## General design - identity

Identity is based on the common, standard Internet - domain names and 
certificates. Users and services have a hostname, with a standard https://
certificate. 

They use diffent confusing words - did, handle, etc - but ultimately the
identity choice is to use FQDN and URLs, so costinm.bsky.social instead 
of costinm@bsky.social, which is the other and more common for of identity.

The identity is backed by standard p256 keypairs, discovered over https.
( the 'bad parts' doc may cover k256 and PLC, and the custom crypto in the protocol).

Hostnames instead of 'user@host' is not a new idea - in the past it didn't
work very well due to the technical difficulties and costs of
managing the certificates/DNS - but with ACME and 'on-demand
cert provisioning' (caddy is used in one of the server implementations) - it
has become easier. It is also a great start for having each user own a 
FQDN where other content can be hosted.

This is IMO the the main 'innovation token' - give each user a domain and 
a key pair, but well hidden from non-technical users. 

The domain and key will enable any company supporting ATproto to provide
additional (paid) services on top - web hosting and all the managed services
that are typically focused on technical users.

## Trust 

Each user or service has a key pair - and all content is signed. The ancient
PGP used the same model - and is still used in git and linux distros - but it 
was never able to go beyond tech people. Bsky just transparently 
gives all users a private/public key and hides all the compexity.

The 'anchor' of identity trust is DNS and domain ownership - there is no
 trust in the profile claims - but everyone can buy or use a free domain, 
 so users build reputation by social means, and in particular the chain
 of followers. You can select to see only content from the people you
 follow, and custom 'feeds' can be created to take advantage of the 
public info about following, using the 'distance' - similar to how PGP 
web of trust was built.

The feed building and trust can use LLM and other means to filter out 
content or organize it in many ways.

Separating the 'trust' building from the rest - with the identity/keypair 
and public feed and followers public - enables 3rd parties to add value and
will make the network survive if BSky dissapears.

## Data representation

It's pretty much git (and many others), but as usual with twists that
makes it different and not interoperable with other 'content ID' and
signature systems.

Each post and 'record' is encoded, the SHA of the content is used as 
content ID and signed, with a mechanism to sync the entire repository.

It is possible to bridge it to other protocols and networks - but this
does not preserve the [signatures](https://atproto.com/specs/repository#repo-data-structure-v3)
since they are applied to the specific CBOR encoding of each record. 
The bridge can apply its own signature on the translated records if the
target protocol/storage has a mechanism to verify (delegation of trust).

The good news is that other protocols and formats can be added - with 
or without the cooperation of BSky, by PDS and alternative UI implementations,
in particular those bridging to other social or private networks.


## Public services

Bsky provides a number of services - they can be replicated and other
organizations may build their own similar or extended services. The public
free services provided by Bsky enable the broad adoption -
the fact that Bsky can dissapear or be replaced without losing
identity (for users using their own domains) or content is also building
trust with users who have been burned out by other social networks.

The core services are:

- the 'identity directory' - build from the known IDs of users and follower graph,
and caching the public keys. Certificate Transparency for public certs is similar
in concept, and other directories can be built if someone wants to pay (and it's
unlikely to be expensive)

- the web app - it supports private storage out of box

- the crawler - that's likely the most expensive part, and will hopefully 
be replaced by a pubsub ( or bridged into one or more pubsub systems).

- a set of feeds - other orgs can build their own, based on the data in the
identity (followers) and crawler.

- abuse / moderation tooling, labeling - can also be built by others.

## Mapping ATproto to K8S and Istio

This is the main reason I'm interested in ATproto. Using this as an example,
any other 'enterprise' mesh and infra can be used.

I think it can be very valuable to build a private K8S PDS, associated with 
each K8S Service, ServiceAccount and APIserver - each getting a did:web handle
and keypair along with storage for signed CRDs as 'posts'.

In K8S and Istio, the public DNS and public certs are replaced with a cluster 
or mesh-wide version, controlled by the cluster or mesh admin. It should 
be relatively easy to deploy a (modified) ATproto server ('PDS') using 
the 'cluster.local' or 'mesh.internal' domains and cluster
resolver. Adding the mesh root to the 'public cert roots' would reduce the 
changes needed in the code, but at least the golang SDK appears easy to
modify to use the mesh roots.

A 'cluster wide' PDS - as well as a per-node PDS - could expose the Pod
and service account identities using the AT proto and DID, and enable the 
use of the SDK and various UIs - using programmatic feeds mirroring the 
API server. It can also be used as an identity/OAuth2 adapter - but 
probably better to stick with the workload identity, enabling P256.

Note that a 'PDS' is not required to support ALL features (may skip the k256
and all the other dubious parts) while still working with a library and
apps that fully support the protocol. The PDS can also implement additional
protocols - exposing the same data in other formats. "Inter" in internet 
is about communication between very different networks, using many 
imperfect standards

Of course, a K8S/Istio based PDS can also be used for users as a private social
network, with a patched mobile and web UI. The users can communicate with each
other - but could also see and modify the infrastructure, with associated signatures
and traceability.

Most of this is possible without Atproto - but unlikely to happen since building
consensus and a new protocol would require a lot of luck and work. 