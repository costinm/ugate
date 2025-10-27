package ugatecaddy

import (
	"context"
	"log/slog"
	"net"
	"net/http"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyevents"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/costinm/ssh-mesh/nio"
	"github.com/costinm/ugate/appinit"
)

func init() {
	// The model is that for each hook a separate module needs to be registered.

	//
	caddy.RegisterModule(UGate{})

	// App
	caddy.RegisterModule(ConfigApp{})
	// App - old style
	caddy.RegisterModule(ConfigApp{ID: "config"})

	// Allows ugate modules to receive events
	caddy.RegisterModule(UGateEventHandler{})
}

// Wrappers to reduce deps on caddy, using generic interfaces.

// healthy, tls events only for now (from what I've seen)
type UGateEventHandler struct {
	l      *slog.Logger
	ctx    caddy.Context
	events *caddyevents.App
	ugate  *ConfigApp
}

func (UGateEventHandler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "events.handlers.ugate",
		New: func() caddy.Module { return new(UGateEventHandler) },
	}
}

// Only after the app is started !
func (g *UGateEventHandler) Handle(ctx context.Context, event caddy.Event) error {
	g.l.Info("Event ", "event", event)
	return nil
}

func (g *UGateEventHandler) Provision(ctx caddy.Context) error {
	g.l = ctx.Slogger()
	g.ctx = ctx

	a, _ := ctx.App("ugate")
	// At this point modules are not yet loaded
	g.l.Info("Provisioning UGate EH", "mods", ctx.Modules(), "app", a)
	return nil
}

// -----------
type ConfigApp struct {
	ID string
	appinit.ResourceStore
	l      *slog.Logger
	ctx    caddy.Context
	events *caddyevents.App
}

// ModuleType returns the Caddy module information.
func (c ConfigApp) CaddyModule() caddy.ModuleInfo {
	if c.ID == "" {
		c.ID = "ugate"
	}
	return caddy.ModuleInfo{
		ID: caddy.ModuleID(c.ID),
		New: func() caddy.Module {
			return &ConfigApp{}
		},
	}
}

func (g *ConfigApp) Provision(ctx caddy.Context) error {
	// ctx.App("name") - uses cfg.apps and LoadModuleByID
	// ctx.Filesystems()

	//ctx.GetMetricsRegistry() - this should be a module or app

	// ctx.IdentityCredentials()
	// ctx.OnCancel()

	// ctx.LoadModule(any, field) will provision a module in a field.
	// ctx.LoadModuleById(id, raw) will call New() and provision.
	//

	g.l = ctx.Slogger()
	g.ctx = ctx

	nio.NewListener = func(ctx context.Context, addr string) net.Listener {
		na, _ := caddy.ParseNetworkAddress(addr)
		l, _ := na.Listen(ctx, 0, net.ListenConfig{})
		return l.(net.Listener)
	}

	g.ResourceStore.Provision(ctx)
	//ctx.Value(any)any

	// List caddy modules:
	return nil
}

func (g *ConfigApp) Start() error {
	for _, mm := range g.ctx.Modules() {
		slog.Info("Caddy modules", "m", mm.CaddyModule().ID)
	}

	//g.events.Emit(g.ctx, "ugate.start", nil)
	eventAppIface, err := g.ctx.App("events")
	if err != nil {
		g.l.Error("Failed to get events", "err", err)
	} else {
		g.events = eventAppIface.(*caddyevents.App)
		g.events.On("ugate.start", g)
	}

	//cm, _ := caddy.GetModule("ugate")

	// a slog handler is registered with zap that just prints the message as info...
	// This is pretty unfortunate/stupid - both are structured.
	slog.Info("bad slog before fix")

	slog.SetDefault(g.l)

	//slog.Info("!!! WORKS", "name", g.Name)

	g.events.Emit(g.ctx, "ugate.start", map[string]any{"app": "hello"})

	return g.ResourceStore.Start()
}

func (g *ConfigApp) Stop() error {
	return nil
}

// This is called because we register explicitly.
func (g *ConfigApp) Handle(ctx context.Context, event caddy.Event) error {
	g.l.Info("UGate handle event", "e", event)
	return nil
}

// ---------

type UGateHandler struct {
	name    string
	handler http.Handler
}

// Handler wraps a http handler - usually as the final step.
func Handler(name string, handler http.Handler) caddy.Module {
	return &UGateHandler{
		name:    name,
		handler: handler,
	}
}

// ModuleType returns the Caddy module information.
func (ugh UGateHandler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: caddy.ModuleID("http.handler." + ugh.name),
		New: func() caddy.Module {
			return &UGateHandler{
				name:    ugh.name,
				handler: ugh.handler,
			}
		},
	}
}

func (g UGateHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request, handler caddyhttp.Handler) error {
	g.handler.ServeHTTP(writer, request)
	return nil
}

// ---------

type UGate struct {
	ctx caddy.Context
}

// ModuleType returns the Caddy module information.
func (UGate) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "ugateall",
		New: func() caddy.Module { return new(UGate) },
	}
}

// dns.providers - for ACME resolution
// func (g UGate) AppendRecords(ctx context.Context, zone string, recs []libdns.Record) ([]libdns.Record, error) {
// 	return nil, nil
// }

// func (g UGate) DeleteRecords(ctx context.Context, zone string, recs []libdns.Record) ([]libdns.Record, error) {
// 	return nil, nil
// }

// caddy.fs - file system
// func (g UGate) Open(name string) (fs.File, error) {
// 	return nil, nil
// }

// http.reverse_proxy.transport
func (g UGate) RoundTrip(request *http.Request) (*http.Response, error) {
	return nil, nil
}

// Select implements the reverseproxy.Selector interface.
// func (g UGate) Select(pool reverseproxy.UpstreamPool, request *http.Request, writer http.ResponseWriter) *reverseproxy.Upstream {
// 	return nil
// }

func (g *UGate) Provision(ctx caddy.Context) error {
	ctx.Slogger().Info("Provisioning UGate")
	g.ctx = ctx

	return nil
}

var (
	_ caddy.Module        = (*UGateEventHandler)(nil)
	_ caddyevents.Handler = (*UGateEventHandler)(nil)

	//_ fs.FS                 = (*UGate)(nil)
	//_ reverseproxy.Selector = (*UGate)(nil)

	_ caddyhttp.MiddlewareHandler = (*UGateHandler)(nil)

	_ http.RoundTripper = (*UGate)(nil)

	//_ certmagic.DNSProvider = (*UGate)(nil)
)
