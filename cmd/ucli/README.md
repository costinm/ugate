# U(micro or universl?) CLI

I believe CLI and cobra are too complicated and lead to bad code if it is designed around
the recommended patterns.

All functionality should be expressed as modules (or 'tools') , with input and output expressible as OpenAPI  and used as a service. 

The CLI is just another way to execute one module - or perhaps a sequence of modules. 
The CLI should be able to run the linked in module - or connect to a running server
that implements a long-lived and possible isolated version of the module. 

There are several applications that spawn a daemon (which may exit when idle) to take 
advantage of caching and longer connections when multiple operations are run. Or to run
in a sandboxed environment instead of running as the user.

## Cobra 

Cobra is the most used flag and CLI package - and is very nice on generating help, autocomplete. The common practice is to manually define the structure - there are 
few packages that auto-generate. 

## Kubectl

Kubectl is a great example of univerals CLI backed by OpenAPI. 

oapi-codegen can be used to generate structs.

# gron

Gron converts json to key/values and back.
https://github.com/TomNomNom/gron


Syntax is valid JS - using properties if valid, or ["key-needing-quote"] :

json[0].commit.author = {};
json[0]["foo-bar"].test = 1;

The CLI supports file, http.

"--json" returns json stream: `[["X-Cloud-Trace-Context"],"c70f7bf26661c67d0b9f2cde6f295319/13941186890243645147"]`

# Nushell

- it's own language - "def {}", etc
- has internal commands but can run systsem ones (^ls)
- rust
- simplified json without quotes
- all commands accept and emit json, but can parse other formats too.
