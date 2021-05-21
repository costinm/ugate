package auth

import (
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

// PlatformInit will attempt to get the identity from the environment.
//
// - K8S - /var/run/secrets/kubernetes.io/serviceaccount/token
// - Istio - extract trust domain from /var/run/secrets/...
// - GCP/CloudRun/K8S - use metadata server to find the service account
// - Regular VMs - look for google 'default credentials' and ~/.kube/config
func PlatformInit() {

}

func (a *Auth) extractMetadata() {
	tok  := GetMDSIDToken("istiod.istio-system.svc")
	if tok == "" {
		return
	}
}

// GetMDS returns MDS info on google:
// instance/hostname - node name.c.PROJECT.internal
// instance/attributes/cluster-name, cluster-location
// project/project-id, numeric-project-id
// instance/service-accounts/ - default, PROJECTID.svc.id.goog
// instance/service-accounts/default/identity - requires the iam.gke.io/gcp-service-account=gsa@project annotation and IAM
// instance/service-accounts/default/token - access token for the KSA
//
func GetMDS(aud string) string {
	req, _ := http.NewRequest("GET",
		"http://metadata.google.internal/computeMetadata/v1/" + aud, nil)
	req.Header.Add("Metadata-Flavor", "Google")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	rb, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return ""
	}
	return string(rb)
}


func (a *Auth) extractK8sJWT() {
	for _, tf := range []string{} {
		if _, err := os.Stat("./var/run/secrets/kubernetes.io/serviceaccount/token"); !os.IsNotExist(err) {
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
