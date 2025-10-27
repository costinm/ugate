# Notes on Go templates

## Helm and the spring library

Seems reasonable to expose the same functions for compat.

## OpenAPI tool calling 

A template needs to be isolated and have minimal direct access to the system, but 
can call stubs for external tools. For modular monoliths - the tools can be local 
calls.

## Permissions and tokens

A template should never have access to secrets, however the tool calling can use 
credentials - locally or on an egress proxy. 

Which secrets/identities and services are allowed from a template needs to be declared
and treated as egress IAM.

## Caddy templates

Caddy has pioneered a lot of interesting features, like the on-demand cert provisioning.
Template support is extensive, and it is using its module library to add dynamic functions:

```
templates [<matcher>] {
	mime    <types...>
	between <open_delim> <close_delim>
	root    <path>
}

Module: "http.handlers.templates"
{▾
	"file_root": "",
	"mime_types": [""],
	"delimiters": [""],
	"match": {•••}
}
```

`mime` allows the template to return json or yaml - not only html.
`between` defaults to golang {{ }}
`root` allow templates to access files on the local disk.

- All sprig functions are supported.

`include` can pass args, available as {{ .Args 0 }}
`{{ Cookie "name" }}`
`{{ env "env_var }}`

`{{placeholder "http.request.uri.path"}}`
`{{ import FILE}}` - defines are visible
listFiles - relative to root, not clear if json is possible
markdown - renders markdown, param is the string
readFile - content of the file, can be piped
.Req - request 
.RespHeader.Set
splitFrontMatter - the front matter is available as .Meta, body as .Body - input can be a file
stripHtml

match setting adds modules to the function map, by module name.

## Functions

- sprig
- slimsprig