package auth

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

const (
	metaPrefix     = "/computeMetadata/v1"
	projIDPath     = metaPrefix + "/project/project-id"
	projNumberPath = metaPrefix + "/project/numeric-project-id"
	instIDPath     = metaPrefix + "/instance/id"
	instancePath   = metaPrefix + "/instance/name"
	zonePath       = metaPrefix + "/instance/zone"
	attrKey        = "attribute"
	attrPath       = metaPrefix + "/instance/attributes/{" + attrKey + "}"
)

// Get an ID token from platform (GCP, etc)
// curl "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/identity?audience=[AUDIENCE]" \
//  -H "Metadata-Flavor: Google"
//
// May fail and need retry
func GetMDSIDToken(ctx context.Context, aud string) (string, error) {
	uri := fmt.Sprintf("instance/service-accounts/default/identity?audience=%s", aud)
	if UseMDSFullToken { // TODO: test the difference
		uri = uri + "&format=full"
	}
	tok, err := MetadataGet(ctx, uri)
	if err != nil {
		return "", err
	}
	return tok, nil
}

const UseMDSFullToken = true

// GetMDS returns MDS info on google:
// instance/hostname - node name.c.PROJECT.internal
// instance/attributes/cluster-name, cluster-location
// project/project-id, numeric-project-id
// instance/service-accounts/ - default, PROJECTID.svc.id.goog
// instance/service-accounts/default/identity - requires the iam.gke.io/gcp-service-account=gsa@project annotation and IAM
// instance/service-accounts/default/token - access token for the KSA
//
func MetadataGet(ctx context.Context, path string) (string, error) {
	mdsHost := os.Getenv("GCE_METADATA_HOST")
	if mdsHost == "" {
		mdsHost = "169.254.169.254" // or metadata.google.internal
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "http://"+mdsHost+"/computeMetadata/v1/"+path, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Metadata-Flavor", "Google")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("metadata server responeded with code=%d %s", resp.StatusCode, resp.Status)
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), err
}

// MDS emulates the GCP metadata server.
// MDS address is 169.254.169.254:80 - can be intercepted with iptables, or
// set using GCE_METADATA_HOST
//
// gRPC library will use it if:
// - the env variable is set
// - a probe to the IP and URL / returns the proper flavor.
// - DNS resolves metadata.google.internal to the IP
func MDS(w http.ResponseWriter, r *http.Request) {
	// WIP
	if !strings.HasPrefix(r.RequestURI, metaPrefix) {
		return
	}
	w.Header().Add("Metadata-Flavor", "Google")

	flavor := r.Header.Get("Metadata-Flavor")
	if flavor == "" && r.RequestURI != "/" {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		return
	}

	switch r.RequestURI {
	case projIDPath:
	case projNumberPath:
	case zonePath:
	}
}
