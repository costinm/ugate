package cmd

import (
	"net/http"

	"github.com/costinm/ugate/appinit"
	"github.com/logdyhq/logdy-core/logdy"
	"github.com/logdyhq/logdy-core/utils"
)

// CLI: --append-to-file=
// --max-message-count
// -t (print fallthrough)

// Tracing:
// - start, duration, correlation_id

// logdy socket 8233
//  tail -f file.log | nc 8233 ( or  logdy forward <port>_)
//
// logdy follow --full-read file.log file2.loggy

type Logdy struct {
	logdy.Config
}

func (l *Logdy) RegisterMux(mux *http.ServeMux) {
	logd := logdy.InitializeLogdy(logdy.Config{HttpPathPrefix: "/.logs", AnalyticsEnabled: false, MaxMessageCount: 10000,
		LogInterceptor: func(entry *utils.LogEntry) {
		}}, mux)

	logd.LogString("Hello world")
}

func init() {
	appinit.RegisterT("logdy", &Logdy{})
}
