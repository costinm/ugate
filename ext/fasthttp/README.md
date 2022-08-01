Experimenting with fasthttp and fasthttp2 - not an option due to lack 
of streaming.

WS may be a good idea, but not worth adding fasthttp dep just for WS

- major issue: no streaming, just RTT
- benefit: low allocs, better perf: https://github.com/night-codes/go-web-framework-benchmark

Components:

- https://github.com/qiangxue/fasthttp-routing - 
  - routing.Context - extends fasthttp.RequestCtx, plus Param() for URL params, Get/Set, WriteData
  - radix tree, like httprouter
  - router.To(methods, path, handlers...)
  - handlers can abort, executed in order - chaining
  - 


Others:

- 
- https://github.com/panjf2000/gnet - based on previous, faster ?
  - thread pool: https://github.com/panjf2000/ants
- gofiber.io - full framework with middleware
  - uses fasthttp
