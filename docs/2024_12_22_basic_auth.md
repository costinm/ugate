# JWT based 'basic' auth

Basic auth typically means user + password. 

Reusing a password is clearly bad practice - you need to generate unique random keys for each domain. That's also called 'apikey', to avoid the stink surrounding 'password'.

Remembering random keys doesn't work - so using you need a password manager.  The password manager needs to be synced and doesn't usually work in all clients.

Let's assume each user has a private key, used for
SSH or mTLS authentication - or in ATproto.

We can generate a signature on (audience,username) and use it as apikey/password - no longer need to store unique random numbers. 

On server side, usually you need some user preferences or data - so some database or filesystem
is needed. Storing the apikey is not a huge burden, 
and can be exchanged with a cookie or JWT (Oauth) so
further calls don't need lookup.


WebAuthn works in a similar way - but it is more complicated. 


# Hidden user name

The 'username' is also complicated: you can't usually use the same name (it may already be taken) and you don't want to use the same name on all sites - for privacy. But you do want
friends to find you and remember your ID. 

For a bank or shop - you need to provide
address and far more, so the real name is a good idea.

For email - something based on your name is useful
for friends to remember.

Let's assume the email is the 'main ID'.

For everything else: a hash as ID. Note that  DID:PLC is mapped back to a handle/domain, that 
has the name we want to keep private in the first place. 

Using SHA(domain,mainID) hides the ID - but the site can still probe, if this is broadly used.


Random people calling or sending messages is not really a desirable feature - 
quite the opposite. Friends-of-friends getting the username may be a feature or not, but it is hard to prevent - however all communication systems should allow 'only friends' or 'fof' as option.


# Tools

- navigator.credentials
- webauthn

# Terms

- 'rp' 'relay part'
