# Conditional compilation modules

To plugin extra protocols, add a small adapter in this directory. 

If it needs more code - a package with separate go.mod can be used.

If the integration has complex dependencies or is specific to a platform - 
it can just be imported from specialized main() apps. GVisor/LWIP for TUN 
support are examples.
