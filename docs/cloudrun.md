# Using uGate in CloudRun

Right now CR services can't discover their instances, nor 
direct traffic to a sticky instance.

However CR allows setting 'max instance' to 1, and registering 
multiple services.

Given uGate behavior, one option is to deploy N 'discovery services',
named ugate0 ... ugateN, and an arbitrary number of connection servers.

The connection servers would maintain connections to each 'discovery',
and send upstream messages.

In very small deployments you only need ugate0, then it can scale 
itself up and act as a meta-server.

There are other combinations possible - including running the discovery
in a k8s cluster, as a stateful set.

Using revision tags is also possible.
