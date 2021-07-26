package oidc

import (
	"context"
	"fmt"
	"strings"

	"github.com/coreos/go-oidc"
)


// JWTRule - from istio API, as json.
//
type JWTRule struct {

	// Example: https://foobar.auth0.com
	// Example: 1234567-compute@developer.gserviceaccount.com
	Issuer string `protobuf:"bytes,1,opt,name=issuer,proto3" json:"issuer,omitempty"`

	// The list of JWT
	// [audiences](https://tools.ietf.org/html/rfc7519#section-4.1.3).
	// that are allowed to access. A JWT containing any of these
	// audiences will be accepted.
	//
	// The service name will be accepted if audiences is empty.
	//
	// Example:
	//
	// ```yaml
	// audiences:
	// - bookstore_android.apps.example.com
	//   bookstore_web.apps.example.com
	// ```
	Audiences []string `protobuf:"bytes,2,rep,name=audiences,proto3" json:"audiences,omitempty"`

	// URL of the provider's public key set to validate signature of the
	// JWT. See [OpenID Discovery](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata).
	//
	// Optional if the key set document can either (a) be retrieved from
	// [OpenID
	// Discovery](https://openid.net/specs/openid-connect-discovery-1_0.html) of
	// the issuer or (b) inferred from the email domain of the issuer (e.g. a
	// Google service account).
	//
	// Example: `https://www.googleapis.com/oauth2/v1/certs`
	//
	// Note: Only one of jwks_uri and jwks should be used. jwks_uri will be ignored if it does.
	JwksUri string `protobuf:"bytes,3,opt,name=jwks_uri,json=jwksUri,proto3" json:"jwks_uri,omitempty"`

	// JSON Web Key Set of public keys to validate signature of the JWT.
	// See https://auth0.com/docs/jwks.
	//
	// Note: Only one of jwks_uri and jwks should be used. jwks_uri will be ignored if it does.
	//Jwks string `protobuf:"bytes,10,opt,name=jwks,proto3" json:"jwks,omitempty"`

	// List of header locations from which JWT is expected. For example, below is the location spec
	// if JWT is expected to be found in `x-jwt-assertion` header, and have "Bearer " prefix:
	// ```
	//   fromHeaders:
	//   - name: x-jwt-assertion
	//     prefix: "Bearer "
	// ```
	//FromHeaders []*JWTHeader `protobuf:"bytes,6,rep,name=from_headers,json=fromHeaders,proto3" json:"from_headers,omitempty"`
	// List of query parameters from which JWT is expected. For example, if JWT is provided via query
	// parameter `my_token` (e.g /path?my_token=<JWT>), the config is:
	// ```
	//   fromParams:
	//   - "my_token"
	// ```
	//FromParams []string `protobuf:"bytes,7,rep,name=from_params,json=fromParams,proto3" json:"from_params,omitempty"`

	// This field specifies the header name to output a successfully verified JWT payload to the
	// backend. The forwarded data is `base64_encoded(jwt_payload_in_JSON)`. If it is not specified,
	// the payload will not be emitted.
	// OutputPayloadToHeader string `protobuf:"bytes,8,opt,name=output_payload_to_header,json=outputPayloadToHeader,proto3" json:"output_payload_to_header,omitempty"`

	// If set to true, the orginal token will be kept for the ustream request. Default is false.
	//ForwardOriginalToken bool     `protobuf:"varint,9,opt,name=forward_original_token,json=forwardOriginalToken,proto3" json:"forward_original_token,omitempty"`
}


type JwtAuthenticator struct {
	trustDomain string
	audiences   []string
	verifier    *oidc.IDTokenVerifier
}

// newJwtAuthenticator is used when running istiod outside of a cluster, to validate the tokens using OIDC
// K8S is created with --service-account-issuer, service-account-signing-key-file and service-account-api-audiences
// which enable OIDC.
func NewJwtAuthenticator(jwtRule JWTRule, trustDomain string) (*JwtAuthenticator, error) {
	issuer := jwtRule.Issuer
	jwksURL := jwtRule.JwksUri
	// The key of a JWT issuer may change, so the key may need to be updated.
	// Based on https://godoc.org/github.com/coreos/go-oidc#NewRemoteKeySet,
	// the oidc library handles caching and cache invalidation. Thus, the verifier
	// is only created once in the constructor.
	var verifier *oidc.IDTokenVerifier
	if len(jwksURL) == 0 {
		// OIDC discovery is used if jwksURL is not set.
		provider, err := oidc.NewProvider(context.Background(), issuer)
		// OIDC discovery may fail, e.g. http request for the OIDC server may fail.
		if err != nil {
			return nil, fmt.Errorf("failed at creating an OIDC provider for %v: %v", issuer, err)
		}
		verifier = provider.Verifier(&oidc.Config{SkipClientIDCheck: true})
	} else {
		keySet := oidc.NewRemoteKeySet(context.Background(), jwksURL)
		verifier = oidc.NewVerifier(issuer, keySet, &oidc.Config{SkipClientIDCheck: true})
	}
	return &JwtAuthenticator{
		trustDomain: trustDomain,
		verifier:    verifier,
		audiences:   jwtRule.Audiences,
	}, nil
}

const (
	// IdentityTemplate is the SPIFFE format template of the identity.
	IdentityTemplate = "spiffe://%s/ns/%s/sa/%s"
)

func (j *JwtAuthenticator) Authenticate(ctx context.Context, bearerToken string) ([]string, error) {
	idToken, err := j.verifier.Verify(ctx, bearerToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify the JWT token (error %v)", err)
	}

	sa := &JwtPayload{}

	// "aud" for trust domain, "sub" has "system:serviceaccount:$namespace:$serviceaccount".
	// in future trust domain may use another field as a standard is defined.
	if err := idToken.Claims(&sa); err != nil {
		return nil, fmt.Errorf("failed to extract claims from ID token: %v", err)
	}

	if !strings.HasPrefix(sa.Sub, "system:serviceaccount") {
		return nil, fmt.Errorf("invalid sub %v", sa.Sub)
	}

	parts := strings.Split(sa.Sub, ":")
	ns := parts[2]
	ksa := parts[3]
	if !checkAudience(sa.Aud, j.audiences) {
		return nil, fmt.Errorf("invalid audiences %v", sa.Aud)
	}

	return []string{fmt.Sprintf(IdentityTemplate, j.trustDomain, ns, ksa)}, nil
}

// checkAudience() returns true if the audiences to check are in
// the expected audiences. Otherwise, return false.
func checkAudience(audToCheck []string, audExpected []string) bool {
	for _, a := range audToCheck {
		for _, b := range audExpected {
			if a == b {
				return true
			}
		}
	}
	return false
}

type JwtPayload struct {
	// Aud is the expected audience, defaults to istio-ca - but is based on istiod.yaml configuration.
	// If set to a different value - use the value defined by istiod.yaml. Env variable can
	// still override
	Aud []string `json:"aud"`

	// Exp is not currently used - we don't use the token for authn, just to determine k8s settings
	Exp int `json:"exp"`

	// Issuer - configured by K8S admin for projected tokens. Will be used to verify all tokens.
	Iss string `json:"iss"`

	Sub string `json:"sub"`
}

