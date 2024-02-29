package http

import (
	"net"
	"net/http"
	"strconv"

	"github.com/costinm/meshauth"
)

// WIP: Normal http listener for a gateway.
//
// Most of the gate is concerned with L4 and L4S - routing on a hostname or VIP.
//
//

type HttpRoutes struct {

	PortRoutes map[string]*ListenerRoutes

	TargetHandlers map[string]http.Handler
}

type ListenerRoutes struct {

	Listener *meshauth.PortListener

	DefaultRoute *HttpRoute

	Routes       map[string]*HttpRoute `json:"routes,omitempty"`

	Mux *http.ServeMux
}

type HttpRoute struct {
	// Listener
	ParentRef string

	// ForwardTo where to forward the proxied connections.
	// Used for accepting on a dedicated port. Will be set as MeshCluster in
	// the stream, can be mesh node.
	// host:port format.
	ForwardTo string `json:"forwardTo,omitempty"`
}


func (ug *ListenerRoutes) FindRoutePrefix(dstaddr net.IP, p uint16, prefix string) *HttpRoute {
	port := ":" + strconv.Itoa(int(p))
	l := ug.Routes[prefix+dstaddr.String()+port]
	if l != nil {
		return l
	}

	l = ug.Routes[prefix+port]
	if l != nil {
		return l
	}

	l = ug.Routes[prefix+"-"+port]
	if l != nil {
		return l
	}
	return ug.DefaultRoute
}
