package acme

import (
	"context"
	"crypto"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	// Used by smallstep
	"github.com/go-chi/chi/v5"
	//"github.com/smallstep/certificates/ca"

	"github.com/smallstep/certificates/acme"
	acmeNoSQL "github.com/smallstep/certificates/acme/db/nosql"
	"github.com/smallstep/certificates/authority"
	"github.com/smallstep/certificates/authority/provisioner"
	"github.com/smallstep/certificates/db"

	"github.com/smallstep/certificates/acme/api"
	"github.com/smallstep/nosql"
)

// Integrate with step-ca, to provide CA services for the mesh.

// Copyright 2015 Matthew Holt and The Caddy Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.


// Handler is an ACME server handler.
type Handler struct {
	// The ID of the CA to use for signing. This refers to
	// the ID given to the CA in the `pki` app. If omitted,
	// the default ID is "local".
	CA string `json:"ca,omitempty"`

	// The lifetime for issued certificates
	//Lifetime caddy.Duration `json:"lifetime,omitempty"`

	// The hostname or IP address by which ACME clients
	// will access the server. This is used to populate
	// the ACME directory endpoint. If not set, the Host // header of the request will be used.
	// COMPATIBILITY NOTE / TODO: This property may go away in the
	// future. Do not rely on this property long-term; check release notes.
	Host string `json:"host,omitempty"`

	// The path prefix under which to serve all ACME
	// endpoints. All other requests will not be served
	// by this handler and will be passed through to
	// the next one. Default: "/acme/".
	// COMPATIBILITY NOTE / TODO: This property may go away in the
	// future, as it is currently only required due to
	// limitations in the underlying library. Do not rely
	// on this property long-term; check release notes.
	PathPrefix string `json:"path_prefix,omitempty"`


	// Specify the set of enabled ACME challenges. An empty or absent value
	// means all challenges are enabled. Accepted values are:
	// "http-01", "dns-01", "tls-alpn-01"
	Challenges []string `json:"challenges,omitempty" `

	// The policy to use for issuing certificates
	//Policy *Policy `json:"policy,omitempty"`


	acmeDB        acme.DB
	acmeAuth      *authority.Authority
	acmeClient    acme.Client
	acmeLinker    acme.Linker

	acmeEndpoints http.Handler
}

func  toSmallstepType(c []string) []provisioner.ACMEChallenge {
	if len(c) == 0 {
		return nil
	}
	ac := make([]provisioner.ACMEChallenge, len(c))
	for i, ch := range c {
		ac[i] = provisioner.ACMEChallenge(ch)
	}
	return ac
}

// Provision sets up the ACME server handler.
func (ash *Handler) Provision(rootCert *x509.Certificate, rootKey crypto.Signer, interm *x509.Certificate, path string) error {

	// set some defaults
	if ash.CA == "" {
		ash.CA = "local"
	}
	if ash.PathPrefix == "" {
		ash.PathPrefix = "/acme/"
	}
	ash.Challenges = []string{"http-01", "dns-01", "tls-alpn-01"}

	database, err := ash.openDatabase(path)
	if err != nil {
		return err
	}

		authConfig := &authority.AuthConfig{
			Provisioners: provisioner.List{
				&provisioner.ACME{
					Name:       ash.CA,
					Challenges: toSmallstepType(ash.Challenges),
					//Options: &provisioner.Options{
					//	X509: ash.Policy.normalizeRules(),
					//},
					Type: provisioner.TypeACME.String(),
					Claims: &provisioner.Claims{
						MinTLSDur:     &provisioner.Duration{Duration: 5 * time.Minute},
						MaxTLSDur:     &provisioner.Duration{Duration: 24 * time.Hour * 365},
						DefaultTLSDur: &provisioner.Duration{Duration: time.Duration(24 * time.Hour)},
					},
				},
			},
		}

	// set up the signer; cert/key which signs the leaf certs
	var signerOption authority.Option
	if interm == nil {
		signerOption = authority.WithX509Signer(rootCert, rootKey)
	} else {
		// if we're signing with intermediate, we need to make
		// sure it's always fresh, because the intermediate may
		// renew while Caddy is running (medium lifetime)
		signerOption = authority.WithX509SignerFunc(func() ([]*x509.Certificate, crypto.Signer, error) {
			issuerKey := rootKey.(crypto.Signer)
			return []*x509.Certificate{interm}, issuerKey, nil
		})
	}

	opts := []authority.Option{
		authority.WithConfig(&authority.Config{
			AuthorityConfig: authConfig,
		}),
		signerOption,
		authority.WithX509RootCerts(rootCert),
	}

	opts = append(opts, authority.WithDatabase(database))
	auth, err := authority.NewEmbedded(opts...)

	ash.acmeAuth = auth
	if err != nil {
		return err
	}

	ash.acmeDB, err = acmeNoSQL.New(database.(nosql.DB))
	if err != nil {
		return fmt.Errorf("configuring ACME DB: %v", err)
	}

	ash.acmeClient, err = ash.makeClient()
	if err != nil {
		return err
	}

	ash.acmeLinker = acme.NewLinker(
		ash.Host,
		strings.Trim(ash.PathPrefix, "/"),
	)

	// extract its http.Handler so we can use it directly
	r := chi.NewRouter()
	r.Route(ash.PathPrefix, func(r chi.Router) {
		api.Route(r)
	})
	ash.acmeEndpoints = r

	return nil
}

func (ash Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
		acmeCtx := acme.NewContext(
			r.Context(),
			ash.acmeDB,
			ash.acmeClient,
			ash.acmeLinker,
			nil,
		)
		acmeCtx = authority.NewContext(acmeCtx, ash.acmeAuth)
		r = r.WithContext(acmeCtx)

		ash.acmeEndpoints.ServeHTTP(w, r)
}



func (ash Handler) openDatabase(path string) (db.AuthDB, error) {

	if path == "" {
		return db.New(nil)
	}

	err := os.MkdirAll(path, 0o755)

	if err != nil {
		return nil, fmt.Errorf("making folder for CA database: %v", err)
	}

	dbConfig := &db.Config{
		Type:       "bbolt",
		DataSource: path,
	}

	// Passing nil returns the simple db
	database, err := db.New(dbConfig)
	return database, err
}

func (ash Handler) makeClient() (acme.Client, error) {
	var resolver *net.Resolver
	resolver = net.DefaultResolver

	return resolverClient{
		Client:   acme.NewClient(),
		resolver: resolver,
	}, nil
}

type resolverClient struct {
	acme.Client

	resolver *net.Resolver
}

func (c resolverClient) LookupTxt(name string) ([]string, error) {
	return c.resolver.LookupTXT(context.Background(), name)
}

