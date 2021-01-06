package k8s

// WIP: add a TCP Gateway spec and the matching routes.
// Gateway is keyed by: [address, port] and can dispatch by hostname
//
// It is expected this is already filtered and matched by the control plane,
// the gateway gets the raw config it needs to apply.
//
// For HTTPRoute: ugate does not process http except hostname, instead a TCPRoute to a
// HTTP gateway (egress GW) can be used.
//
// PortListener objects are associated with addr:port defined in the gateway, collapsing all
// configs. Each Gateway adds to the map of host->route, so requests can be mapped to a
// route object. Label selection is done by the control plane or tool configuring this -
//
// The 'dialer' object should handle routing - will get the hostname and port extracted from
// the metadata (port is based on listener port or meta), and info in context about which
// gateway terminated.
//

