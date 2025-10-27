package main


// Small CLI for authentication and core mesh functionality.
// - detect the environment and show it
// - init keys for CA and workload
// - get JWTs from the keys
// - get JWTs from K8S, MDS, google
// - decode and verify JWT
// - encrypt and decrypt webpush
// - tunnel over h2c

/*
  After an unfortunate bloat-ware and NIH trend started by Docker's
  'one process per container', it appears we are going back to the Unix model
  of tools doing speicific jobs and using processes.

  The ugate package is using Cobra and reflection or OpenAPI schemas to build the CLI.

  This package is dependency-free and is using a simpler approach - taking advantage of
  the 'resource' registry. The registry is a map of 'kind' to struct, and the struct can
  implement the Run interface.
*/


// Command line client - using the cobra commands
func main() {
	CobraExecute()
}

