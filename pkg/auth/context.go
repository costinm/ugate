package auth

// ReqContext is a context associated with a request.
import (
	"context"
	"net"
	"time"
)

// Typically for H2:
// 	h2ctx := r.Context().Value(mesh.H2Info).(*mesh.ReqContext)
type ReqContext struct {
	// Auth role - set if a authorized_keys or other authz is configured
	Role string

	// SAN list from the certificate, or equivalent auth method.
	SAN []string

	// Request start time
	T0 time.Time

	// Public key of the first cert in the chain (similar with SSH)
	Pub []byte

	// VIP associated with the public key.
	VIP net.IP

	VAPID *JWT
}

// ID of the caller, validated based on certs.
// Currently based on VIP6 for mesh nods.
func (rc *ReqContext) ID() string {
	if rc.VIP == nil {
		return ""
	}
	return rc.VIP.String()
}

type h2Key int

var h2Info = h2Key(1)

func AuthContext(ctx context.Context) *ReqContext {
	return ctx.Value(h2Info).(*ReqContext)
}

func ContextWithAuth(ctx context.Context, h2c *ReqContext) context.Context {
	return context.WithValue(ctx, h2Info, h2c)
}
