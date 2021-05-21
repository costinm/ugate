
function dbg_start() {
  dlv --listen=:40000 --headless=true --api-version=2 --accept-multiclient exec /go/bin/ugate
}

function compiled() {
  # CompileDaemon -log-prefix=false -build="go build -o /app" -command="/app"
  #dlv --listen=:40000 --headless=true --api-version=2 --accept-multiclient exec /app
  #https://www.reddit.com/r/golang/comments/i7hvco/compiledaemon_with_delve_debugging_while_auto/

   CompileDaemon -build="echo 1" \
     -command="dlv --listen=:40000 --headless=true --api-version=2 --accept-multiclient ./cmd/ugate"
}

