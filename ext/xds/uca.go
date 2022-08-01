package xds

import (
	"context"

	auth "github.com/costinm/ugate/gen/proto/istio/v1/auth"
)

// UCA is a micro CA, intended for testing/local use. It is a very simplified version of Istio CA, but implemented
// using proxyless gRPC and as a minimal micro-service. It has no dependencies on K8S or Istio - expects the
// root CA to be available in a file ( can be a mounted Secret, or loaded from a secret store ), and expects
// gRPC middleware to handle authentication.
type UCA struct {
}

// TODO
func (uca *UCA) CreateCertificate(context.Context, *auth.IstioCertificateRequest) (*auth.IstioCertificateResponse, error) {
	return nil, nil
}
