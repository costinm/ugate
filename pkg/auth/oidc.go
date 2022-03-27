package auth

import (
	"crypto/ecdsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

// Minimal implementatio of OIDC, matching K8S. Other helpers for platform-specific tokens.


// In GKE, iss: format is https://container.googleapis.com/v1/projects/$PROJECT/locations/$LOCATION/clusters/$CLUSTER
// and the discovery doc is relative (i.e. standard). The keys are
// $ISS/jwks
//
// GCP also uses (https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/blob/v0.2.0/auth/auth.go):
// https://securetoken.googleapis.com/v1/identitybindingtoken
// "serviceAccount:<project>.svc.id.goog[<namespace>/<sa>]"
//
// In Istio, the ID token can be exchanged for access tokens:
// POST https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/
// service-<GCP project number>@gcp-sa-meshdataplane.iam.gserviceaccount.com:generateAccessToken
// Content-Type: application/json
// Authorization: Bearer <federated token>
// {
//  "Delegates": [],
//  "Scope": [
//      https://www.googleapis.com/auth/cloud-platform
//  ],
// }
//
// curl http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/my-gsa@my-project.iam.gserviceaccount.com/token"
// -H'Metadata-Flavor:Google'
//
// Access tokens:
// https://developers.google.com/identity/toolkit/reference/securetoken/rest/v1/token
// POST https://securetoken.googleapis.com/v1/token
// grant_type=authorization_code&code=ID_TOKEN
// grant_type=refresh_token&refresh_token=TOKEN
// Resp: {
//  "access_token": string,
//  "expires_in": string,
//  "token_type": string,
//  "refresh_token": string,
//}
//
//


type discDoc struct {
	Issuer      string   `json:"issuer"`
	AuthURL     string   `json:"authorization_endpoint"`
	TokenURL    string   `json:"token_endpoint"`
	JWKSURL     string   `json:"jwks_uri"`
	UserInfoURL string   `json:"userinfo_endpoint"`
	Algorithms  []string `json:"id_token_signing_alg_values_supported"`
}

type userInfo struct {
	Subject string `json:"sub"`
	Profile string `json:"profile"`
	Email   string `json:"email"`
}

// StsRequestParameters stores all STS request attributes defined in
// https://tools.ietf.org/html/draft-ietf-oauth-token-exchange-16#section-2.1
type StsRequestParameters struct {
	// REQUIRED. The value "urn:ietf:params:oauth:grant-type:token- exchange"
	// indicates that a token exchange is being performed.
	GrantType string
	// OPTIONAL. Indicates the location of the target service or resource where
	// the client intends to use the requested security token.
	Resource string
	// OPTIONAL. The logical name of the target service where the client intends
	// to use the requested security token.
	Audience string
	// OPTIONAL. A list of space-delimited, case-sensitive strings, that allow
	// the client to specify the desired Scope of the requested security token in the
	// context of the service or Resource where the token will be used.
	Scope string
	// OPTIONAL. An identifier, for the type of the requested security token.
	RequestedTokenType string
	// REQUIRED. A security token that represents the identity of the party on
	// behalf of whom the request is being made.
	SubjectToken string
	// REQUIRED. An identifier, that indicates the type of the security token in
	// the "subject_token" parameter.
	SubjectTokenType string
	// OPTIONAL. A security token that represents the identity of the acting party.
	ActorToken string
	// An identifier, that indicates the type of the security token in the
	// "actor_token" parameter.
	ActorTokenType string
}

type StsResponseParameters struct {
	// REQUIRED. The security token issued by the authorization server
	// in response to the token exchange request.
	AccessToken string `json:"access_token"`
	// REQUIRED. An identifier, representation of the issued security token.
	IssuedTokenType string `json:"issued_token_type"`
	// REQUIRED. A case-insensitive value specifying the method of using the access
	// token issued. It provides the client with information about how to utilize the
	// access token to access protected resources.
	TokenType string `json:"token_type"`
	// RECOMMENDED. The validity lifetime, in seconds, of the token issued by the
	// authorization server.
	ExpiresIn int64 `json:"expires_in"`
	// OPTIONAL, if the Scope of the issued security token is identical to the
	// Scope requested by the client; otherwise, REQUIRED.
	Scope string `json:"scope"`
	// OPTIONAL. A refresh token will typically not be issued when the exchange is
	// of one temporary credential (the subject_token) for a different temporary
	// credential (the issued token) for use in some other context.
	RefreshToken string `json:"refresh_token"`
}

const (
	// TokenExchangeGrantType is the required value for "grant_type" parameter in a STS request.
	TokenExchangeGrantType = "urn:ietf:params:oauth:grant-type:token-exchange"
	// SubjectTokenType is the required token type in a STS request.
	SubjectTokenType = "urn:ietf:params:oauth:token-type:jwt"

)

// OIDC discovery on .well-known/openid-configuration
func (a *Auth) HandleDisc(w http.ResponseWriter, r *http.Request) {
	// Issuer must match the hostname used to connect.
	//
	w.Header().Set("content-type", "application/json")
	fmt.Fprintf(w, `{
  "issuer": "https://%s",
  "jwks_uri": "https://%s/jwks",
  "response_types_supported": [
    "id_token"
  ],
  "subject_types_supported": [
    "public"
  ],
  "id_token_signing_alg_values_supported": [
    "ES256"
  ]
}`, r.Host, r.Host)

	// ,"EdDSA"
  // TODO: switch to EdDSA
}

// OIDC JWK
func (a *Auth) HandleJWK(w http.ResponseWriter, r *http.Request) {
	pk := a.Cert.PrivateKey.(*ecdsa.PrivateKey)
	byteLen := (pk.Params().BitSize + 7) / 8
	ret := make([]byte, byteLen)
	pk.X.FillBytes(ret[0: byteLen])
	x64 := base64.RawURLEncoding.EncodeToString(ret[0:byteLen])
	pk.Y.FillBytes(ret[0: byteLen])
	y64 := base64.RawURLEncoding.EncodeToString(ret[0:byteLen])
	fmt.Fprintf(w, `{
  "keys": [
    {
		 "kty" : "EC",
		 "crv" : "P-256",
		 "x"   : "%s",
		 "y"   : "%s",
    }
  ]
	}`, x64, y64)

	//		"crv": "Ed25519",
	//		"kty": "OKP",
	//		"x"   : "11qYAYKxCrfVS_7TyWQHOg7hcvPapiMlrwIaaPcHURo",
}

// RFC8693 - token exchange
// ex. for GCP: https://cloud.google.com/iam/docs/reference/sts/rest/v1beta/TopLevel/token
// https://cloud.google.com/iam/docs/reference/credentials/rest
//
func (a *Auth) HandleSTS(w http.ResponseWriter, req *http.Request) {
	if parseErr := req.ParseForm(); parseErr != nil {
		w.WriteHeader(400)
		return
	}
	reqParam := &StsRequestParameters{}
	reqParam.GrantType = req.PostForm.Get("grant_type")
	reqParam.Resource = req.PostForm.Get("resource")
	reqParam.Audience = req.PostForm.Get("audience")
	reqParam.Scope = req.PostForm.Get("scope")
	reqParam.RequestedTokenType = req.PostForm.Get("requested_token_type")
	reqParam.SubjectToken = req.PostForm.Get("subject_token")
	reqParam.SubjectTokenType = req.PostForm.Get("subject_token_type")
	reqParam.ActorToken = req.PostForm.Get("actor_token")
	reqParam.ActorTokenType = req.PostForm.Get("actor_token_type")

	// TODO:
}

// See golang.org/x/oauth2/google

func (a *Auth) LoadManaged() {
	addr := os.Getenv("KUBERNETES_SERVICE_HOST")
	if addr != "" {
		// Running in K8S - use the env
		port := os.Getenv("KUBERNETES_SERVICE_PORT")
		if port == "" {
			port = "443"
		}
		namespace, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if err != nil {
			return
		}
		cl := &KubeNamedCluster{Name: "default", Cluster: KubeCluster{Server: "https" + addr + ":" + port,
			CertificateAuthority: "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"}}
		u := &KubeNamedUser{Name:"defaultm", User: KubeUser{TokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token"}}

		log.Println("In-cluster: ", namespace, u,  cl)
	}

	adc := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	if adc != "" {
		b, err := ioutil.ReadFile(adc)
		if err == nil {
			cf := &credentialsFile{}
			err = json.Unmarshal(b, cf)
			// TODO: use refresh token and token_uri ("https://accounts.google.com/o/oauth2/token")
		}
	}

}

func GetToken(aud string) string {
	res, err := http.Get("http://169.254.169.254/instance/service-accounts/default/identity?audience=" + aud)
	if err != nil {
		return ""
	}
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return ""
	}
	return string(b)
}

// JSON key file types.
const (
	serviceAccountKey  = "service_account"
	userCredentialsKey = "authorized_user"
)

// credentialsFile is the unmarshalled representation of a credentials file.
type credentialsFile struct {
	Type string `json:"type"` // serviceAccountKey or userCredentialsKey

	// Service Account fields
	ClientEmail  string `json:"client_email"`
	PrivateKeyID string `json:"private_key_id"`
	PrivateKey   string `json:"private_key"`
	TokenURL     string `json:"token_uri"`
	ProjectID    string `json:"project_id"`

	// User Credential fields
	// (These typically come from gcloud auth.)
	ClientSecret string `json:"client_secret"`
	ClientID     string `json:"client_id"`
	RefreshToken string `json:"refresh_token"`
}
