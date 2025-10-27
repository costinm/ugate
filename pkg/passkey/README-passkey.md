# Passkey

TODO:
- try out the emulator - https://github.com/bulwarkid/bulwark-passkey
In particular: USB-over-Tcp+mesh is interesting idea, useful for VMs/mesh

- test few android BLE tokens

- can it be used by regular mesh apps (for service accounts, not users) ?
https://github.com/bodik/soft-webauthn  

## Concepts

Server side (RP, Relying party):
- http server - store public keys and IDs (maybe)
- sends 'CredentialCreationOptions' (should ask user presence, etc) and 32B nonce/challenge
- gets PublicKeyCredentail - verify signed credentials
- registration creates the key on the authenticator, mapped to RP URL.

If "ID is the public key" - RP doesn't need to store anything, policies are
set on the public key hash directly (can generate a FQDN or email using the pubkey).


Client side 
- browser - show form, interact with the provider
  - provider - signs the tokens.
- non-browser: generate equivalent signatures using 
  - a 'provider' ( bluethooth, etc )
  - local keys

# Web Authn

TL;DR: the browser (or client) is assigned a public key for authn, and can
sign stuff. The server can use the public key as identity for the client and may also encrypt using the key.


# Using key pairs for encryption and authn

Assuming each participant has a key pair, and the public key is known to the other parties it is possible to use an insecure transport - like SMTP or HTTP, as well
as git or files - with payloads that are encrypted and signed.


## Webpush

Webpush is a similar mechanism - you can specify the public key of the server, for VAPID, and you get a public key that can be used to send push messages. In has some 'notifications allowed'  semantics which make it hard to use for browsers or phones, unless notifications are a normal feature, but the rest of the protocol applies.

PushManager takes an optional server public key, returns a public key.

WebAuthn uses the server FQDN - which can be associated with a public key using DID or the server cert (if direct comms are used and dedicated cert).

In both cases we can expect a pair of public keys as identity and for encryption.

## Objects

Options:
```yaml
publicKey:
  challenge: [32Base64URL]
  pubKeyCredParams:
    alg: 7
    type: public-key
  attestations: "none" # or missing
  rp:
    id: example.internal
    name: example 
  user:
    id: user@example.internal
  attestation: direct # require validated user ID
```

Registration data - effectively just the public key, rest is reflected:
```yaml
rawId: [Client credential ID]
type: public-key
clientDataJSON:

attestationObject: # cbor()
  fmt: none
  attStmt: {}
  authData: rp_id_hash + \x41 (attested_data + user present) + sign_count + aaguid + len(credential_id) + credential_id + cbor(PUBLICKEY)
```

Credential data:
```yaml
authenticatorData:
  SHA256(rp.id) + flag(\x01) + SIGN_COUNT
clientDataJSON:
  type: webauthn.get # or .create when it is created
  challenge: CHALLENGE
  origin: ORIGIN
signature: SIGN(authenticatorData + SHA256(clientDataJson))
```