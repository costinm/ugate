# AT protocol imperfections

As a 'social federated network' there is no need to invent a new JSON schema, use custom DID, strange formats.

Using DNS-anchored identity and well-hidden keypairs is a great decision and what makes ATproto work,
and also a redeeming feature - but really hope in time 'optional features' can be added by community 
in custom PDS and maybe adopted by Bsky, and gradually replace the custom ones. 

In particular:
- DER and certificates in identity
- standard, interoperable Oauth2 - accept other providers, generate tokens usable with clouds (https aud).
- just sign each record... 

## Intro

First thing when reading a protocol is understanding the purpose and 
terminology - and translating all the novel invented words into
common concepts.

ATproto invents a lot of useless terms - 'lexicon' for their NIH schema, 'handle' and 'did' for URL.

I should note that IMO blockchain is a great concept and very useful where needed, with many
legitimate applications, including financial. The scams around cryptocurrency are a big problem
and create a very negative light on a lot of technology in this space. Yes, government is bad
and fiat money will keep inflating - but random dudes issuing their own fake money and conning 
regular people into pyramid schemes is far worse than any government, and any crypto will go
fast to zero the moment the governments regulate and prevent exchange to fiat, which will happen
after one big bust.

## Crypto

Non standard crypto... It's a social network, absolutely no need to mess with crypto.

[Example](https://github.com/DavidBuchanan314/millipds/blob/main/src/millipds/crypto.py)

## Identity

Non standard formats. The PLC idea is not bad - nor is the 'public key server', PGP has used this for decades.

[Example](https://github.com/DavidBuchanan314/millipds/blob/main/src/millipds/crypto.py)

Improvements: support DER/PEM certificate, and allow users to present a certificate - with the original key as root.
This can still be done, at least for new users if the private keys are used to generate certs.

The standard is anchored in public certs and DNS - all did:web resolution uses ACME certs. They could
just generate a EC256 key, get an ACME cert with the handle (DNS SAN - without did:web), and use that 
as the basis of the PLC.

Note that recovery key is far less useful than it looks - if you lose the domain name you may be able 
to keep the private key as identity anyways, but the handle will change.

Fix: for 'alternate' PDS implementations, use ACME/DER as an alternative to the PLC. And while at it - also 
generate SSH and PGP keys with the same format, and register them with github and PGP keyservers. The key
is your identity - why not have it broadly available, or use the private key you already have ?

## OAuth

Not compatible with common servers - would be very nice to allow user to sign in from github or clouds to
link their identities or not require them to store refresh tokens. The point of OAuth is to allow interop 
and federation - not to create a locked-down authentication island (main problem is the DID in aud/sub).

Fix: custom PDS could easily accept standard JWTs and implement this (in addition to the custom did)

## DID

It's really a subset/cleanup of URL. If you change the user@domain identity format - you can also define
a 'base32(key).mesh' DNS-compatible domain for keys, so everything is a FQDN and no need for DID, in particular
if the main DID used is non-standard and exclusive to atproto.

Fix: UX does not expose the plc did, and custom PDS servers can use FQDN (DNS SANs, etc).

## Storage


### Encoding 

CBOR is an IETF standard - DAG-CBOR sounds like CBOR, but removes all tags - just to add a single custom TAG back,
for CID.

At least a standard CBOR impl could (with care) encode, and it can decode.

Fix: DER (or protobuf) could be used with alternative APIs by custom PDS servers and clients (based on some negotiation).

### CAR file

Another cryptocurrency NIH - it is not a bad format in terms of overhead, just concatenated key/values blocks.
It is not worse than git PACK format - which is more complex and also NIH.

To be fair - 'tar'/cpio file has huge header overhead and is designed for files, and there is not much else 
with broad adoption.

I would have picked LevelDB format, and exposed an API to lookup by CID.

Good news: having a PDS support additional data formats is relatively easy.

