# Envoy

- applies to all LBs

- set-cookie with the address of upstream host
- if cookie present, use that header
- upstream host address is base64 encoded
- 

  api/envoy/extensions/filters/http/stateful_session/v3/stateful_session.proto

{"cookie", fmt::format("global-session-cookie=\"{}\"",
Envoy::Base64::encode("127.0.0.1:50002", 15))}};


```yaml

  name: envoy.filters.http.stateful_session
  typed_config:
    "@type": type.googleapis.com/envoy.extensions.filters.http.stateful_session.v3.StatefulSession
    session_state:
      name: envoy.http.stateful_session.cookie
      typed_config:
        "@type": type.googleapis.com/envoy.extensions.http.stateful_session.cookie.v3.CookieBasedSessionState
        name: global-session-cookie
        path: /path
        ttl: 120s
```

# Original DST:

x-envoy-original-dst-host header  - can be set

{
"use_http_header": ...,
"http_header_name": ...
}


https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/load_balancing/original_dst#arch-overview-load-balancing-types-original-destination

Filter state: 
envoy.network.transport_socket.original_dst_address

