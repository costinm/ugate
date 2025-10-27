//go:build !MIN
// +build !MIN

package cmd

import (
	"github.com/costinm/ugate/appinit"
	k8s "github.com/costinm/mk8s"
)

// Local receiver for XDS.
// May proxy to one or more XDS servers.
func init() {
	appinit.RegisterT[k8s.K8S]("k8s", &k8s.K8S{})
	//
	//	// Experimental K8S-style API
	//	//module.Mesh.Mux.HandleFunc("/apis/meshauth.io/v1/namespaces/", mdbs.HandleK8S)

	//	// Experimental Github-like SSH keys info. Username and /keys processed by handler
	//	// Gitea uses /api/v1/ prefix but requires authz.
	//	// Result is array, with "key" in each obj as an authorized key.
	//	//module.Mesh.Mux.HandleFunc("/users/", mdbs.HandleK8S)
	//	return nil
	//})
}
