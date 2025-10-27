package cmd

import (
	"context"
	"log"
	"time"

	k8sc "github.com/costinm/mk8s"
	"github.com/costinm/mk8s/pkg/csrctrl"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
)


func CSRSignD(ctx context.Context, k *k8sc.K8S) {

	is, err := k.Default.Client().CoreV1().Secrets("istio-system").Get(ctx, "cacerts", metav1.GetOptions{})
	if err != nil {
		log.Fatal(err)
	}

	auth := &csrctrl.CertificateAuthority{
		Roots: is.Data["ca.crt"],
		Chain: is.Data["tls.crt"],
		Key: is.Data["tls.key"],
	}

	err = auth.Init()
	if err != nil {
		log.Fatal(err)
	}

	factory := informers.NewSharedInformerFactory(k.Default.Client(), time.Hour*24)

	_ = csrctrl.NewK8SSigner(k.Default.Client(), "test.internal/signer", factory, auth)
	factory.Start(make(<-chan struct{}))

}
