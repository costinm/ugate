package auth

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

// PlatformInit will attempt to get the identity from the environment.
//
// - K8S - /var/run/secrets/kubernetes.io/serviceaccount/token
// - Istio - extract trust domain from /var/run/secrets/...
// - GCP/CloudRun/K8S - use metadata server to find the service account
// - Regular VMs - look for google 'default credentials' and ~/.kube/config

func (a *Auth) extractMetadata() {
	tok, _ := GetMDSIDToken(context.Background(), "istiod.istio-system.svc")
	if tok == "" {
		return
	}
}

func (a *Auth) extractK8sJWT() {
	for _, tf := range []string{} {
		if _, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token"); !os.IsNotExist(err) {
			data, err := ioutil.ReadFile(tf)
			if err != nil {
				continue
			}
			_, t, _, _, _ := JwtRawParse(string(data))
			if t.Iss != "kubernetes/serviceaccount" {
				log.Println("Unexpected iss", t.Raw)
				continue
			}
			subP := strings.Split(t.Sub, ":")
			if strings.HasPrefix(t.Sub, "system:serviceaccount") && len(subP) >= 4 {
				// TODO: use the namespace and KSA
			}
		}
	}
}
