// Copyright 2015 Matthew Holt and The Caddy Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package main is the entry point of the Caddy application.
// Most of Caddy's functionality is provided through modules,
// which can be plugged in by adding their import below.
//
// There is no need to modify the Caddy source code to customize your
// builds. You can easily build a custom Caddy with these simple steps:
//
//  1. Copy this file (main.go) into a new folder
//  2. Edit the imports below to include the modules you want plugged in
//  3. Run `go mod init caddy`
//  4. Run `go install` or `go build` - you now have a custom binary!
//
// Or you can use xcaddy which does it all for you as a command:
// https://github.com/caddyserver/xcaddy
package main

import (
	"fmt"
	"os"

	caddycmd "github.com/caddyserver/caddy/v2/cmd"

	// plug in Caddy modules here
	_ "github.com/caddyserver/caddy/v2/modules/standard"

	// Yaml support built-in via an adapter to json
	_ "github.com/iamd3vil/caddy_yaml_adapter"
	// This one extends with templates and env vars - but bad
	// error messages, in particular if env vars have unexpected
	// things (which is common). Both appear unmaintained.
	// "github.com/abiosoft/caddy-yaml"

	// DNS
	_ "github.com/caddy-dns/cloudflare"
	//_ "github.com/caddy-dns/dnsoverhttps"
	//_ "github.com/caddy-dns/googlecloud"
	_ "github.com/caddy-dns/rfc2136"

	// SSH
	//_ "github.com/kadeessh/kadeessh"

	_ "github.com/mholt/caddy-l4/layer4"
	_ "github.com/mholt/caddy-l4/modules/l4proxy"

	// Allows exec command for events
	_ "github.com/mholt/caddy-events-exec"

	// Adapter to ugate framework - similar to caddy, but
	// without hard deps.
	_ "github.com/costinm/ugate/pkg/ugatecaddy"

	// Modules for ugate - registered directly
	_ "github.com/costinm/ugate/cmd"
)

func main() {

	args := []string{"caddy", "run",
		"--watch",
		"--config", "ugate-caddy.yaml",
		"--adapter", "yaml"}
	os.Args = args
	cwd, _ := os.Getwd()

	fmt.Println("Starting from", cwd)

	caddycmd.Main()
}
