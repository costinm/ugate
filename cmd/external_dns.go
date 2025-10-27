//go:build !no_external_dns
package cmd

import (
	"context"
	"log"
	"time"

	"sigs.k8s.io/external-dns/controller"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/pkg/apis/externaldns"
	"sigs.k8s.io/external-dns/pkg/apis/externaldns/validation"
	"sigs.k8s.io/external-dns/plan"
	"sigs.k8s.io/external-dns/provider"
	"sigs.k8s.io/external-dns/provider/cloudflare"
	"sigs.k8s.io/external-dns/provider/inmemory"
	webhookapi "sigs.k8s.io/external-dns/provider/webhook/api"
	"sigs.k8s.io/external-dns/registry"
	"sigs.k8s.io/external-dns/source"
)

// Integrate with external-dns, to provide DNS resolution for the mesh.

type ExternalDNS struct {
	externaldns.Config

	Source *source.Config

	K8SController controller.Controller
}

type ExternalDNSSrc struct {
	Name string

	// Destination for the DNS records - the webhook provider
	Dest string
}




func RunExternalDNSSource(ctx context.Context, cfg *ExternalDNS) error {

	if err := validation.ValidateConfig(&cfg.Config); err != nil {
		log.Fatal("config validation failed", err)
	}

	cfg.DryRun = true

	sourceCfg := cfg.Source

	sources, err := source.ByNames(ctx, &source.SingletonClientGenerator{
		KubeConfig:   cfg.KubeConfig,
		APIServerURL: cfg.APIServerURL,
		// If update events are enabled, disable timeout.
		RequestTimeout: func() time.Duration {
			if cfg.UpdateEvents {
				return 0
			}
			return cfg.RequestTimeout
		}(),
	}, cfg.Sources, sourceCfg)
	if err != nil {
		log.Fatal(err)
	}

	// Filter targets
	targetFilter := endpoint.NewTargetNetFilterWithExclusions(cfg.TargetNetFilter, cfg.ExcludeTargetNets)

	// Combine multiple sources into a single, deduplicated source.
	endpointsSource := source.NewDedupSource(source.NewMultiSource(sources, sourceCfg.DefaultTargets))
	endpointsSource = source.NewTargetFilterSource(endpointsSource, targetFilter)

	// Provider will be external - just URL configured.

	// RegexDomainFilter overrides DomainFilter
	var domainFilter endpoint.DomainFilter
	if cfg.RegexDomainFilter.String() != "" {
		domainFilter = endpoint.NewRegexDomainFilter(cfg.RegexDomainFilter, cfg.RegexDomainExclusion)
	} else {
		domainFilter = endpoint.NewDomainFilterWithExclusions(cfg.DomainFilter, cfg.ExcludeDomains)
	}
	//zoneNameFilter := endpoint.NewDomainFilter(cfg.ZoneNameFilter)
	//zoneIDFilter := provider.NewZoneIDFilter(cfg.ZoneIDFilter)
	//zoneTypeFilter := provider.NewZoneTypeFilter(cfg.AWSZoneType)
	//zoneTagFilter := provider.NewZoneTagFilter(cfg.AWSZoneTagFilter)

	p := inmemory.NewInMemoryProvider(inmemory.InMemoryInitZones(cfg.InMemoryZones), inmemory.InMemoryWithDomain(domainFilter), inmemory.InMemoryWithLogging())

	if err != nil {
		log.Fatal(err)
	}

	var r registry.Registry
	switch cfg.Registry {
	case "noop":
		r, err = registry.NewNoopRegistry(p)
	default:
		r, err = registry.NewTXTRegistry(p, cfg.TXTPrefix, cfg.TXTSuffix, cfg.TXTOwnerID, cfg.TXTCacheInterval, cfg.TXTWildcardReplacement, cfg.ManagedDNSRecordTypes, cfg.ExcludeDNSRecordTypes, cfg.TXTEncryptEnabled, []byte(cfg.TXTEncryptAESKey))
	}

	policy, exists := plan.Policies["sync"]
	if !exists {
		log.Fatalf("unknown policy: %s", cfg.Policy)
	}

	ctrl := controller.Controller{
		Source:               endpointsSource,
		Registry:             r,
		Policy:               policy,
		Interval:             cfg.Interval,
		DomainFilter:         domainFilter,
		ManagedRecordTypes:   cfg.ManagedDNSRecordTypes,
		ExcludeRecordTypes:   cfg.ExcludeDNSRecordTypes,
		MinEventSyncInterval: cfg.MinEventSyncInterval,
	}
	cfg.K8SController = ctrl
	return nil
}

// Run the sync only once, used for CI/CD
func (e *ExternalDNS) Once(ctx context.Context)	error {
	err := e.K8SController.RunOnce(ctx)
	return err
}

func (e *ExternalDNS) Run(ctx context.Context)	error {

	ctrl := e.K8SController

	// Add RunOnce as the handler function that will be called when ingress/service sources have changed.
	// Note that k8s Informers will perform an initial list operation, which results in the handler
	// function initially being called for every Service/Ingress that exists
	e.K8SController.Source.AddEventHandler(ctx, func() { ctrl.ScheduleRunOnce(time.Now()) })

	ctrl.ScheduleRunOnce(time.Now())
	ctrl.Run(ctx)

	return nil
}

// Run one DNS provider plus the webhook server.
func RunExternalDNSProvider(ctx context.Context, cfg *ExternalDNSProvider) error {
	var domainFilter endpoint.DomainFilter
	//if cfg.RegexDomainFilter.String() != "" {
	//	domainFilter = endpoint.NewRegexDomainFilter(cfg.RegexDomainFilter, cfg.RegexDomainExclusion)
	//} else {
	domainFilter = endpoint.NewDomainFilterWithExclusions(cfg.DomainFilter, cfg.ExcludeDomains)
	//}
	p := inmemory.NewInMemoryProvider(inmemory.InMemoryInitZones(cfg.InMemoryZones), inmemory.InMemoryWithDomain(domainFilter), inmemory.InMemoryWithLogging())

	webhookapi.StartHTTPApi(p, nil, cfg.WebhookProviderReadTimeout, cfg.WebhookProviderWriteTimeout, ":8080")

	return nil
}



func RunCFDNSProvider(ctx context.Context, cfg *ExternalDNSProvider) error {
	var domainFilter endpoint.DomainFilter
	//if cfg.RegexDomainFilter.String() != "" {
	//	domainFilter = endpoint.NewRegexDomainFilter(cfg.RegexDomainFilter, cfg.RegexDomainExclusion)
	//} else {
	domainFilter = endpoint.NewDomainFilterWithExclusions(cfg.DomainFilter, cfg.ExcludeDomains)
	//}
	zoneIDFilter := provider.NewZoneIDFilter(cfg.ZoneIDFilter)

	p, err := cloudflare.NewCloudFlareProvider(domainFilter, zoneIDFilter, false, false, 100)
	if err != nil {
		return err
	}

	webhookapi.StartHTTPApi(p, nil, cfg.WebhookProviderReadTimeout, cfg.WebhookProviderWriteTimeout, ":8080")

	return nil
}
