package main

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/costinm/meshauth"
	"github.com/costinm/meshauth/pkg/init"
	"github.com/costinm/meshauth/pkg/mdsd"
	"github.com/costinm/meshauth/pkg/tokens"
	k8sc "github.com/costinm/mk8s"
)

// Meshauth-agent  can run:
// - on a local dev machine
// - in a docker container
// - as K8S deamonet set
// - as sidecar
//
// MDS Will emulate a google MDS server to provide tokens and meta.
//
// TODO: It also handles ext_authz http protocol from Envoy.
// TODO: maintain k8s-like JWT and cert to emulate in-cluster
//
// Source of auth:
// - kube config with token (minimal deps) or in-cluster - for running in K8S
//
// - TODO: MDS in a VM/serverless - with permissions to the cluster
//
//	Non-configurable port 15014 - iptables should redirect port 80 of the MDS.
//
// iptables -t nat -A OUTPUT -p tcp -m tcp -d 169.254.169.254 --dport 80 -j REDIRECT --to-ports 15014
//
// For envoy and c++ grpc - requires /etc/hosts or resolver for metadata.google.internal.
// 169.254.169.254 metadata.google.internal.
//
// Alternative: use ssh-mesh or equivalent to forward to real MDS.
func main() {
	ctx := context.Background()

	//

	// Will use ~/.ssh for keys and base configs, load uagent.json containing MeshConfig
	ma, err := meshauth.FromEnv(ctx, "uagent")

	// Register K8S - the agent is using a GKE kube config as bootstrap
	// Namespace will be defaulted from config
	// This is using the official client.
	k := k8sc.Default()
	if k.Default == nil {
		log.Fatal("K8S required")
	}

	// "gcp_fed" returns access tokens
	// "gcp" returns GCP signed JWTS for the GSA - only if a GSA is set.
	// "k8s" provider returns K8S signed JWTs
	// First 2 are used for access tokens (gcp used if a GSA exists).
	// Last 2 used for id tokens (gcp used if a GSA exists)
	p, _, _ := GcpInfo(k.Default.Name)
	pid := p

	mainMux := http.NewServeMux()

	mdd := ugcp.NewServer()
	mdd.Mux = mainMux

	mdd.TokenProvider = k // k8s tokens for default account

	fedS := tokens.NewFederatedTokenSource(&tokens.STSAuthConfig{
		AudienceSource: pid + ".svc.id.goog",
		TokenSource:    k,
	})

	// Federated access tokens (for ${PROJECT_ID}.svc.id.goog[ns/ksa]
	// K8S JWT access tokens otherwise.
	mdd.GCPTokenProvider = fedS

	gsa := ""
	if mdd.Metadata.Instance.ServiceAccounts != nil {
		gsa = mdd.Metadata.Instance.ServiceAccounts["default"].Email
	}
	if gsa == "" {
		// Use default naming conventions
		gsa = "k8s-" + ma.Namespace + "@" + pid + ".iam.gserviceaccount.com"
	}

	if ma.GSA != "-" {
		audTokenS := tokens.NewFederatedTokenSource(&tokens.STSAuthConfig{
			TokenSource:    k,
			GSA:            gsa,
			AudienceSource: pid + ".svc.id.goog",
		})
		ma.AuthProviders["gcp"] = audTokenS
	}
	// We need to load some config here...
	cp, err := k.Default.GetCM(ctx, "default", "mds")
	if err != nil {
		log.Println(cp)
	}

	mdd.Start()
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		// Old Istio Mixer port (for authz)
		err := http.ListenAndServe("0.0.0.0:15014", mainMux)
		if err != nil {
			log.Fatal(err)
		}
	}()
	appinit.WaitEnd()
}

// GcpInfo extracts project and id from gke or connectgateway cluster name.
func GcpInfo(cf string) (string, string, string) {
	// TODO: also parse the URL form
	if strings.HasPrefix(cf, "gke_") {
		parts := strings.Split(cf, "_")
		if len(parts) > 3 {
			// TODO: if env variable with cluster name/location are set - use that for context
			return parts[1], parts[2], parts[3]
		}
	}
	if strings.HasPrefix(cf, "connectgateway_") {
		parts := strings.Split(cf, "_")
		if len(parts) > 2 {
			// TODO: if env variable with cluster name/location are set - use that for context
			// TODO: if opinionanted naming scheme is used for cg names (location-name) - extract it.

			// TODO: use registration names that include the location !

			return parts[1], "", parts[2]
		}
	}
	return "", "", cf
}
