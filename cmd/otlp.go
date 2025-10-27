package cmd

func init() {
		//ot := otel.NewOtel(&otel.Config{})
		//
		//ot.Register(mux)

	//
	//meshauth.Register("otel", func(m *meshauth.Module) error {
	//	// Wrapper around otel SDK with some defaults. Detects from env various settings,
	//	// default tracing to OTLP if possible or slog if not.
	//	ot := otelbootstrap.Get()
	//	m.Module = ot
	//
	//	m.Mesh.RTWrapper = ot.HttpClient
	//	m.Mesh.HandlerWrapper = ot.HttpHandler
	//
	//	return nil
	//})

	//meshauth.Register("otlp-prom", func(m *meshauth.Module) error {
	//	// ===== Telemetry =================
	//	if m.Address == "" {
	//		// Istio MetricReader port - should not be intercepted.
	//		m.Address = ":15020"
	//	}
	//
	//	// TODO: use the settings to determine tel destination.
	//	// TODO: resource should be based on the identity SHA1 as main hostname key.
	//
	//	// The exporter embeds a default OpenTelemetry Reader and
	//	// implements prometheus.Collector, allowing it to be used as
	//	// both a Reader and Collector.
	//
	//	// Wrapper around otel SDK with some defaults. Detects from env various settings,
	//	// default tracing to OTLP if possible or slog if not.
	//	ot := otelbootstrap.Get()
	//	ot.Prometheus = func() metric.Reader {
	//		prom, err := otelprom.New()
	//		if err != nil {
	//			panic(err)
	//		}
	//		return prom
	//	}
	//	ot.InitMetrics()
	//
	//	mux := http.NewServeMux()
	//	// Expose the registered metrics via HTTP.
	//	mux.Handle("/metrics", promhttp.HandlerFor(
	//		prometheus.DefaultGatherer,
	//		promhttp.HandlerOpts{
	//			// Opt into OpenMetrics to support exemplars.
	//			EnableOpenMetrics: true,
	//		},
	//	))
	//
	//	go func() {
	//		err := http.ListenAndServe(m.Address, mux)
	//		if err != nil {
	//			log.Fatal(err)
	//		}
	//	}()
	//
	//	m.Mesh.RTWrapper = ot.HttpClient
	//	m.Mesh.HandlerWrapper = ot.HttpHandler
	//
	//	return nil
	//})

}
