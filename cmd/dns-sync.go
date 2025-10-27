package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/costinm/dns-sync-gcp/pkg/provider/google"
	dns_sync "github.com/costinm/dns-sync/pkg/dns-sync"
	"github.com/costinm/dns-sync/pkg/sources/dnsmesh"
	"github.com/costinm/dns-sync/source"
	"github.com/costinm/ugate/appinit"
)

// Run an external dns source once, sync to in-memory or webhook.

type ExternalDNSProvider struct {
	WebhookProviderReadTimeout  time.Duration
	WebhookProviderWriteTimeout time.Duration

	// Used to filter allowed domains
	DomainFilter   []string
	ExcludeDomains []string

	InMemoryZones []string

	// Used by CF to filter zones
	ZoneIDFilter  []string

	GoogleProject string
}

func init() {
	RegisterGoogleDNS()
}

type DNSDump struct {
	ResourceStore appinit.ResourceStore
}

func (d *DNSDump) Run(ctx context.Context, args []string) error {
	m, _ := d.ResourceStore.Get(context.Background(), "googledns")
	if m == nil {
		return nil
	}

	googlep := m.(*google.GoogleProvider)
	z, err := googlep.Zones(context.Background())
	if err != nil {
		return err
	}

	for k, v := range z {
		fmt.Println("Zone: ", k, v.DnsName, v.Visibility)
	}

	r, err := googlep.Records(context.Background())
	if err != nil {
		return err
	}

	for _, rr := range r {
		fmt.Println("Records: ", rr)
	}


	return nil
}

func RegisterGoogleDNS() {

	// A factory for the provider, which implements handlers

	// varz is an alternative
	appinit.RegisterT[google.GoogleProviderConfig]("googledns", &google.GoogleProviderConfig{})


		//googlep, err := google.NewGoogleProvider(context.Background(), cfg)
		//if err != nil {
		//	return err
		//}
		//dns_service.InitHandlers(googlep, mux, "/google")

  // appinit.RegisterN("dnsdump", )
	appinit.RegisterT("dnsdump", &DNSDump{})
	appinit.RegisterT("dnssync", &DNSSync{})

}

type DNSSync struct {
	Once bool
}

func (d *DNSSync) Run(ctx context.Context, args ...string) error {
	source.SourceFn["mesh-service"] = dnsmesh.NewMeshServiceSource

	// No need to register metrics or signal handling if we're running in once mode.
	// TODO: switch to OTel, generate traces too
	//tel.ServeMetrics()
	err := dns_sync.LoadAndRun(ctx,  d.Once)
	if err != nil {
		return err
	}


	return nil
}
