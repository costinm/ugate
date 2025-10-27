package cmd

import (
	"context"
	"log"
	"time"

	"github.com/costinm/meshauth"
	"github.com/costinm/ugate/appinit"
	"github.com/costinm/mk8s/gcp"
)

// RegisterGCP registers the GCP-related modules
func init() {
	appinit.RegisterT[gcp.GKE]("gcp", &gcp.GKE{})
	//appinit.RegisterAny("gcpdump", DumpGKE)
}

// DumpGKE is a module that dumps the GKE clusters and info
func DumpGKE(ctx context.Context, gke *gcp.GKE) error {

	fromK8S := false

	if gke.K8S.Default != nil {
		log.Println("Found meshconfig/default clusters", gke.K8S.Default.Name, len(gke.K8S.ByName))
		fromK8S = true
	}


	k8s := gke.K8S

	// Explicitly load all clusters (gke.New() selectively loads based on env)
	cl, err := gke.LoadGKEClusters(ctx, "", "")
	if err != nil {
		return err
	}
	log.Println("Found GKE clusters", len(cl))

	// Explicitly load all hub clusters
	cl2, err := gke.LoadHubClusters(ctx, "")
	if err != nil {
		return err
	}
	log.Println("Found HUB clusters", len(cl2))

	if !fromK8S {
		log.Println("Default from GKE clusters", gke.K8S.Default, len(gke.K8S.ByName))
	}

	gke.Autodetect(ctx, "")

	istio_ca, err := gke.GetToken(ctx, "istio_ca")
	if err != nil {
		log.Println("Failed to get GCP token", err)
	} else {
		j := meshauth.DecodeJWT(istio_ca)
		log.Println("K8S Default Token", "k8s", j.String())
	}


	// Can't return JWT tokens signed by google for federated identities, but K8S can.
	access, err := gke.K8S.Default.GetToken(ctx, "istio-ca")
	if err != nil {
		log.Println("Failed to get K8S token", err)
	} else {
		j := meshauth.DecodeJWT(access)
		log.Println("K8S Default Token", "k8s", j.String())

	}

	ch := make(chan bool, 8)
	// Test the clusters
	for kn, kk := range k8s.ByName {
		kn := kn
		kk := kk
		go func() {
			t0 := time.Now()
			v, err := kk.Client().ServerVersion()
			if err != nil {
				log.Println(kn, v, err)
				ch <- false
				return
			}
			d1 := time.Since(t0)
			t0 = time.Now()
			kk.Client().ServerVersion()

			log.Println(kn, v.Major, v.Minor, v.Platform, d1, time.Since(t0))

			//me, err := kk.Client().CoreV1().ConfigMaps("istio-system").Get(ctx, "mesh-env", v1.GetOptions{})
			//if err == nil {
			//	log.Println(kn, me.Data["PROJECT_NUMBER"])
			//} else {
			//	log.Println(kn, err)
			//}
			ch <- true
		}()
	}
	for range k8s.ByName {
		<-ch
	}

	return err
}
