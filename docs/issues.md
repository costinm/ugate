# Open issues 

## Prioritization

uGate ( and Istio BTS, MASQUE ) tunnel multiple multiplexed streams over QUIC
or H2. 

Before, the network layer could prioritize by port - a bad thing on open internet,
but important in a cloud infra.  

For example backup/bulk transfer packets on a specifc port could be dropped 
in case of congestion before customer-related traffic on port 443.

Easiest and common solution is to use do dropping based on port / IP - both 
will be lost if traffic is routed to an egress gateway and muxed.

Example: https://lartc.org/howto/lartc.cookbook.fullnat.intro.html

Possible solution: 
- use different ports for the BTS/MASQUE, and route traffic based on port to 
the different multiplexer ports. In this case network can still drop packets
  on the 'bulk' traffic class.
  
- quic-level prioritization/dropping of streams if congestion is detected.
Can also be based on ports, or richer.
