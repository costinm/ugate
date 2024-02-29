package ugate

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"

	"github.com/costinm/meshauth"
)

// mesh abstracts the 'clusters'/'services'/endpoints and connections.

// MeshCluster represents a set of endpoints, with a common configuration.
// Can be a K8S Service with VIP and DNS name, an external service, etc.
//
// Similar with Envoy Cluster or K8S service, can also represent single
// endpoint with multiple paths/IPs.
// It includes node information, based on registration info or discovery.
//
// Also used for 'mesh' nodes, where we have a public key and other info, as well
// as non-mesh nodes.
//
// This struct includes statistics about the node and current active association/mux
// connections.
type MeshCluster struct {
	// Dest includes the address and auth-related info for the MeshCluster.
	// The meshauth package includes helpers around authentication and certificates, decoupled from mesh.
	meshauth.Dest

	// MeshCluster WorkloadID - the cluster name in kube config, hub, gke - cluster name in XDS
	// Defaults to Base addr - but it is possible to have multiple clusters for
	// same address ( ex. different users / token providers).
	//
	// Examples:
	// GKE cluster: gke_PROJECT_LOCATION_NAME
	//
	// For mesh nodes:
	// ID is the (best) primary id known for the node. Format is:
	//    base32(SHA256(EC_256_pub)) - 32 bytes binary, 52 bytes encoded
	//    base32(ED_pub) - same size, for nodes with ED keys.
	//
	// For non-mesh nodes, it is a (real) domain name or IP if unknown.
	// It may include port, or even be a URL - the external destinations may
	// have different public keys on different ports.
	//
	// The node may be a virtual IP ( ex. K8S/Istio service ) or name
	// of a virtual service.
	//
	// If IPs are used, they must be either truncated SHA or included
	// in the node cert or the control plane must return metadata and
	// secure low-level network is used (like wireguard)
	//
	// Required for secure communication.
	//
	// Examples:
	//  -  [B32_SHA]
	//  -  [B32_SHA].reviews.bookinfo.svc.example.com
	//  -  IP6 (based on SHA or 'trusted' IP)
	//  -  IP4 ('trusted' IP)
	//
	ID string `json:"id,omitempty"`

	// IPFS:
	// http://<gateway host>/ipfs/CID/path
	// http://<cid>.ipfs.<gateway host>/<path>
	// http://gateway/ipns/IPNDS_ID/path
	// ipfs://<CID>/<path>, ipns://<peer WorkloadID>/<path>, and dweb://<IPFS address>
	//
	// Multiaddr: TLV

	// Active connections to endpoints, each is a multiplexed H2 connection.
	//EndpointCon []*EndpointCon

	// Hosts are workload addresses associated with the backend service.
	//
	// If empty, the MeshCluster Addr will be used directly - it is expected to be
	// a FQDN or VIP that is routable - either a service backed by an LB or handled by
	// ambient or K8S.
	//
	// This may be pre-configured or result of discovery (IPs, extra properties).
	Hosts []*Host

	// Parent.
	//UGate *UGate

	// TODO: UserAgent, DefaultHeaders

	// TLS config used when dialing using workload identity, shared per dest.
	//TLSClientConfig *tls.Config

	//LastUsed time.Time


}


// GetCluster returns a cluster for the given address, or nil if not found.
func (hb *UGate) GetCluster(addr string) *MeshCluster {
	hb.m.RLock()
	c := hb.Clusters[addr]
	// Make sure it is set correctly.
	if c != nil && c.ID == "" {
		c.ID = addr
	}

	hb.m.RUnlock()
	return c
}


// Cluster will get an existing cluster or create a dynamic one.
// Dynamic clusters can be GC and loaded on-demand.
func (hb *UGate) Cluster(ctx context.Context, addr string) (*MeshCluster, error) {
	// TODO: extract cluster from addr, allow URL with params to indicate how to connect.
	//host := ""
	//if strings.Contains(dest, "//") {
	//	u, _ := url.Parse(dest)
	//
	//	host, _, _ = net.SplitHostPort(u.Host)
	//} else {
	//	host, _, _ = net.SplitHostPort(dest)
	//}
	//if strings.HasSuffix(host, ".svc") {
	//	hc.H2Gate = hg + ":15008" // hbone/mtls
	//	hc.ExternalMTLSConfig = auth.GenerateTLSConfigServer()
	//}
	//// Initialization done - starting the proxy either on a listener or stdin.

	// 1. Find the cluster for the address. If not found, create one with the defaults or use on-demand
	// if XDS server is configured
	hb.m.RLock()
	c, ok := hb.Clusters[addr]
	hb.m.RUnlock()

	// TODO: use discovery to find info about service addr, populate from XDS on-demand or DNS
	if !ok {
		// TODO: on-demand, DNS lookups, etc
		c = &MeshCluster{Dest: meshauth.Dest{Addr: addr, Dynamic: true}, ID: addr}
		hb.AddCluster(c)
	}
	//c.LastUsed = time.Now()
	return c, nil
}

// AddCluster will add a cluster to be used for Dial and RoundTrip.
// The 'Addr' field can be a host:port or IP:port.
// If id is set, it can be host:port or hostname - will be added as a destination.
// The service can be IP:port or URLs
func (hb *UGate) AddCluster(c *MeshCluster, host ...*Host) *MeshCluster {
	hb.m.Lock()
	hb.Clusters[c.Addr] = c

	if c.ID != "" {
		hb.Clusters[c.ID] = c
	}

	//c.UGate = hb
	if c.ConnectTimeout == 0 {
		c.ConnectTimeout = hb.ConnectTimeout.Duration
	}
	hb.m.Unlock()

	for _, s := range host {
		c.Hosts = append(c.Hosts, s)
	}
	return c
}

//func (hb *UGate) AddDest(c *meshauth.Dest, host ...*Host) *MeshCluster {
//
//	return hb.AddCluster(&MeshCluster{Dest: *c}, host...)
//}



// Textual representation of the node registration data.
func (n *MeshCluster) String() string {
	b, _ := json.Marshal(n)
	return string(b)
}

// Host represents the properties of a single workload.
// By default, clusters resolve the endpoints dynamically, using DNS or EDS or other
// discovery mechanisms.
type Host struct {
	// Labels for the workload. Extracted from pod info - possibly TXT records
	//
	// 'hbone' can be used for a custom hbone endpoint (default 15008).
	//
	Labels map[string]string `json:"labels,omitempty"`

	//LBWeight int `json:"lb_weight,omitempty"`
	//Priority int

	// Address is an IP where the host can be reached.
	// It can be a real IP (in the mesh, direct) or a jump host.
	//
	Address string `json:"addr,omitempty"`

	// FQDN of the host. Used to check host cert.
	Hostname string
}

//func (e *Host) HboneAddress() string {
//	addr := e.Labels["hbone"]
//	if addr != "" {
//		return addr
//	}
//
//	if addr == "" && e.Address != "" {
//		addr = e.Address
//		h, _, _ := net.SplitHostPort(addr)
//		addr = net.JoinHostPort(h, "15008")
//		return addr
//	}
//
//	return addr
//}
//


//// EndpointCon is a multiplexed H2 client for a specific destination instance part
//// of a MeshCluster.
////
//// It wraps a real H2/QUIC implementation (RoundTripper), a connection, config and metadata.
//type EndpointCon struct {
//	Cluster  *MeshCluster
//	Endpoint *Host
//
//	// Multiplex connection - may implement additional interfaces to open lighter
//	// streams, like Dial()
//	RoundTripper http.RoundTripper // *http2.ClientConn or custom (wrapper)
//
//	TLSConn net.Conn
//	// The stream connection - may be a real TCP or not
//	streamCon       net.Conn
//	ConnectionStart time.Time
//	SSLEnd          time.Time
//}

//func (c *MeshCluster) UpdateEndpoints(ep []*Host) {
//	c.UGate.m.Lock()
//	// TODO: preserve unmodified endpoints connections, by IP, refresh pending
//	c.Hosts = ep
//	c.UGate.m.Unlock()
//}

// EndpointCon (peer) is over capacity or unavailable.
var ServiceUnavailable = errors.New("Service Unavailable 503")


// Return the custom cert pool, if cluster config specifies a list of trusted
// roots, or nil if default trust is used.
func (c *MeshCluster) trustRoots() *x509.CertPool {
	return c.Dest.CertPool()
}
//
//func (hc *EndpointCon) Close() error {
//	return hc.TLSConn.Close()
//}
