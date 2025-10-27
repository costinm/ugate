module github.com/costinm/ugate/cmd

go 1.24.2

replace github.com/imdario/mergo => github.com/imdario/mergo v0.3.5

replace github.com/costinm/mk8s => ../../mk8s

replace github.com/costinm/mk8s/gcp => ../../mk8s/gcp

replace github.com/costinm/grpc-mesh => ../../grpc-mesh

replace github.com/costinm/grpc-mesh/gen/proto => ../../grpc-mesh/gen/proto

replace github.com/costinm/ugate => ../

replace github.com/costinm/dns-sync => ../../dns-sync

replace github.com/costinm/dns-sync-gcp => ../../dns-sync-gcp

replace github.com/costinm/ssh-mesh => ../../ssh-mesh

replace github.com/costinm/ugate/pkg/gvisor => ../pkg/gvisor

replace github.com/costinm/utel/otelbootstrap => ../../utel/otelbootstrap

replace github.com/costinm/utel/uotel => ../../utel/uotel

replace github.com/costinm/utel => ../../utel

replace github.com/costinm/ugate/pkg/webrtc => ../pkg/webrtc

replace github.com/costinm/meshauth => ../../meshauth

//Larger buffer, hooks to use the h3 stack
//replace github.com/lucas-clemente/quic-go => ../../../quic
//replace github.com/lucas-clemente/quic-go => github.com/costinm/quic v0.5.1-0.20210425224043-9f67435d0255

require (
	github.com/abiosoft/caddy-yaml v0.0.0-20210522210701-64fbdd07cf02
	github.com/caddy-dns/cloudflare v0.0.0-20240703190432-89f16b99c18e
	github.com/caddy-dns/rfc2136 v0.1.1
	github.com/caddyserver/caddy/v2 v2.9.1
	github.com/caddyserver/certmagic v0.21.7
	github.com/containernetworking/cni v1.2.3
	github.com/costinm/dns-sync v0.0.0-20240803185312-98818804911c
	github.com/costinm/dns-sync-gcp v0.0.0-00010101000000-000000000000
	github.com/costinm/go-tun2socks v1.17.0
	github.com/costinm/grpc-mesh v0.0.0-20240605230615-7973e12eb107
	github.com/costinm/grpc-mesh/gen/connect-go v0.0.0-20240605142039-df0642efa7d0
	github.com/costinm/grpc-mesh/gen/proto v0.0.0-20240605230615-7973e12eb107
	github.com/costinm/meshauth v0.0.0-20240908184618-7f1851595a99
	github.com/costinm/mk8s v0.0.0-20240804162407-08eec2ca6674
	github.com/costinm/mk8s/gcp v0.0.0-20240627225616-dad11d70021b
	github.com/costinm/ssh-mesh v0.0.0-20250317042722-e070eb619d04
	github.com/costinm/ugate v0.0.0-20240229060225-8ac4b465e5d9
	github.com/costinm/utel v0.0.0-00010101000000-000000000000
	github.com/fsnotify/fsnotify v1.8.0
	github.com/go-json-experiment/json v0.0.0-20250124004741-3d76ae074650
	github.com/goccy/go-yaml v1.15.15
	github.com/golang/protobuf v1.5.4
	github.com/google/cel-go v0.21.0
	github.com/gorilla/websocket v1.5.3
	github.com/libdns/libdns v0.2.2
	github.com/logdyhq/logdy-core v0.13.0
	github.com/mholt/caddy-events-exec v0.0.0-20231121214933-055bfd2e8b82
	github.com/mitchellh/mapstructure v1.5.1-0.20220423185008-bf980b35cac4
	github.com/mochi-co/mqtt v1.3.2
	github.com/ollama/ollama v0.3.5
	github.com/pires/go-proxyproto v0.7.1-0.20240628150027-b718e7ce4964
	github.com/songgao/water v0.0.0-20200317203138-2b4b6d7c09d8
	github.com/spf13/cobra v1.8.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.19.0
	golang.org/x/exp v0.0.0-20240904232852-e7e105dedf7e
	golang.org/x/net v0.40.0
	k8s.io/apimachinery v0.30.3
	k8s.io/client-go v0.30.3
	sigs.k8s.io/external-dns v0.14.2
	sigs.k8s.io/yaml v1.4.0
)

require github.com/mastercactapus/proxyprotocol v0.0.4 // indirect

require (
	cloud.google.com/go v0.116.0 // indirect
	cloud.google.com/go/auth v0.12.1 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.6 // indirect
	cloud.google.com/go/compute/metadata v0.5.2 // indirect
	cloud.google.com/go/container v1.40.0 // indirect
	cloud.google.com/go/gkehub v0.15.1 // indirect
	cloud.google.com/go/iam v1.2.1 // indirect
	cloud.google.com/go/logging v1.12.0 // indirect
	cloud.google.com/go/longrunning v0.6.1 // indirect
	cloud.google.com/go/monitoring v1.21.1 // indirect
	dario.cat/mergo v1.0.1 // indirect
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/AndreasBriese/bbloom v0.0.0-20190825152654-46b345b51c96 // indirect
	github.com/BurntSushi/toml v1.4.0 // indirect
	github.com/F5Networks/k8s-bigip-ctlr/v2 v2.17.1 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Masterminds/semver/v3 v3.3.0 // indirect
	github.com/Masterminds/sprig v2.22.0+incompatible // indirect
	github.com/Masterminds/sprig/v3 v3.3.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/alecthomas/chroma/v2 v2.14.0 // indirect
	github.com/alecthomas/kingpin/v2 v2.4.0 // indirect
	github.com/alecthomas/units v0.0.0-20240626203959-61d1e3462e30 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.0 // indirect
	github.com/aryann/difflib v0.0.0-20210328193216-ff5ff6dc229b // indirect
	github.com/aws/aws-sdk-go v1.55.4 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.28.6 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/brianvoe/gofakeit/v6 v6.28.0 // indirect
	github.com/bufbuild/connect-go v1.10.0 // indirect
	github.com/caddyserver/zerossl v0.1.3 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cheggaaa/pb/v3 v3.1.5 // indirect
	github.com/chzyer/readline v1.5.1 // indirect
	github.com/cloudflare/cloudflare-go v0.100.0 // indirect
	github.com/cloudfoundry-community/go-cfclient v0.0.0-20220930021109-9c4e6c59ccf1 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.5 // indirect
	github.com/datawire/ambassador v1.12.4 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dgraph-io/badger v1.6.2 // indirect
	github.com/dgraph-io/badger/v2 v2.2007.4 // indirect
	github.com/dgraph-io/ristretto v0.1.1 // indirect
	github.com/dgryski/go-farm v0.0.0-20200201041132-a6ae2369ad13 // indirect
	github.com/dlclark/regexp2 v1.11.4 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/emersion/go-maildir v0.6.0 // indirect
	github.com/emersion/go-sasl v0.0.0-20241020182733-b788ff22d5a6 // indirect
	github.com/emersion/go-smtp v0.21.3 // indirect
	github.com/emicklei/go-restful/v3 v3.12.1 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/francoispqt/gojay v1.2.13 // indirect
	github.com/fxamacker/cbor/v2 v2.8.0 // indirect
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32 // indirect
	github.com/go-chi/chi/v5 v5.1.0 // indirect
	github.com/go-jose/go-jose/v3 v3.0.3 // indirect
	github.com/go-kit/kit v0.13.0 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.21.0 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-sql-driver/mysql v1.7.1 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/goccy/go-json v0.10.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/glog v1.2.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/google/certificate-transparency-go v1.1.8-0.20240110162603-74a5dd331745 // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/go-tpm v0.9.1 // indirect
	github.com/google/go-tspi v0.3.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/pprof v0.0.0-20240727154555-813a5fbdbec8 // indirect
	github.com/google/s2a-go v0.1.8 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.4 // indirect
	github.com/googleapis/gax-go/v2 v2.14.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.22.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.7 // indirect
	github.com/hashicorp/hcl v1.0.1-vault-5 // indirect
	github.com/huandu/xstrings v1.5.0 // indirect
	github.com/iamd3vil/caddy_yaml_adapter v0.0.0-20200503183711-d479c29b475a
	github.com/imdario/mergo v1.0.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jackc/chunkreader/v2 v2.0.1 // indirect
	github.com/jackc/pgconn v1.14.3 // indirect
	github.com/jackc/pgio v1.0.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgproto3/v2 v2.3.3 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/pgtype v1.14.0 // indirect
	github.com/jackc/pgx/v4 v4.18.3 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/klauspost/cpuid/v2 v2.2.9 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/labstack/gommon v0.4.2 // indirect
	github.com/libdns/cloudflare v0.1.2-0.20240604123710-0549667a10ab // indirect
	github.com/libdns/rfc2136 v0.1.0 // indirect
	github.com/linki/instrumented_http v0.3.0 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/manifoldco/promptui v0.9.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/mholt/acmez/v3 v3.0.1 // indirect
	github.com/mholt/caddy-l4 v0.0.0-20250530154005-4d3c80e89c5f
	github.com/miekg/dns v1.1.62 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-ps v1.0.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/nxadm/tail v1.4.11 // indirect
	github.com/onsi/ginkgo/v2 v2.19.1 // indirect
	github.com/openshift/api v0.0.0-20240724184751-84047ef4a2ce // indirect
	github.com/openshift/client-go v0.0.0-20240528061634-b054aa794d87 // indirect
	github.com/pelletier/go-toml/v2 v2.2.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pkg/sftp v1.13.7 // indirect
	github.com/projectcontour/contour v1.29.1 // indirect
	github.com/prometheus/client_golang v1.20.5 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.55.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/quic-go/qpack v0.5.1 // indirect
	github.com/quic-go/quic-go v0.48.2 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/rs/xid v1.5.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sagikazarmark/locafero v0.4.0 // indirect
	github.com/sagikazarmark/slog-shim v0.1.0 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/shurcooL/sanitized_anchor_name v1.0.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/slackhq/nebula v1.7.2 // indirect
	github.com/smallstep/certificates v0.26.2 // indirect
	github.com/smallstep/go-attestation v0.4.4-0.20240109183208-413678f90935 // indirect
	github.com/smallstep/nosql v0.6.1 // indirect
	github.com/smallstep/pkcs7 v0.0.0-20231024181729-3b98ecc1ca81 // indirect
	github.com/smallstep/scep v0.0.0-20231024192529-aee96d7ad34d // indirect
	github.com/smallstep/truststore v0.13.0 // indirect
	github.com/smartystreets/goconvey v1.7.2 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spf13/afero v1.11.0 // indirect
	github.com/spf13/cast v1.7.0 // indirect
	github.com/stoewer/go-strcase v1.3.0 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/tailscale/tscert v0.0.0-20240608151842-d3f834017e53 // indirect
	github.com/urfave/cli v1.22.15 // indirect
	github.com/valyala/fastjson v1.6.4 // indirect
	github.com/vishvananda/netns v0.0.4 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/xhit/go-str2duration/v2 v2.1.0 // indirect
	github.com/yuin/goldmark v1.7.8 // indirect
	github.com/yuin/goldmark-highlighting/v2 v2.0.0-20230729083705-37449abec8cc // indirect
	github.com/zeebo/blake3 v0.2.4 // indirect
	go.etcd.io/bbolt v1.3.10 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.54.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.56.0 // indirect
	go.opentelemetry.io/contrib/propagators/autoprop v0.42.0 // indirect
	go.opentelemetry.io/contrib/propagators/aws v1.17.0 // indirect
	go.opentelemetry.io/contrib/propagators/b3 v1.21.1 // indirect
	go.opentelemetry.io/contrib/propagators/jaeger v1.19.0 // indirect
	go.opentelemetry.io/contrib/propagators/ot v1.19.0 // indirect
	go.opentelemetry.io/otel v1.33.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.31.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.31.0 // indirect
	go.opentelemetry.io/otel/metric v1.33.0 // indirect
	go.opentelemetry.io/otel/sdk v1.33.0 // indirect
	go.opentelemetry.io/otel/trace v1.33.0 // indirect
	go.opentelemetry.io/proto/otlp v1.3.1 // indirect
	go.step.sm/cli-utils v0.9.0 // indirect
	go.step.sm/crypto v0.47.0 // indirect
	go.step.sm/linkedca v0.21.1 // indirect
	go.uber.org/automaxprocs v1.6.0 // indirect
	go.uber.org/mock v0.4.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	go.uber.org/zap/exp v0.3.0 // indirect
	golang.org/x/crypto v0.38.0 // indirect
	golang.org/x/crypto/x509roots/fallback v0.0.0-20241104001025-71ed71b4faf9 // indirect
	golang.org/x/mod v0.22.0 // indirect
	golang.org/x/oauth2 v0.24.0 // indirect
	golang.org/x/sync v0.14.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/term v0.32.0 // indirect
	golang.org/x/text v0.25.0 // indirect
	golang.org/x/time v0.8.0 // indirect
	golang.org/x/tools v0.27.0 // indirect
	google.golang.org/api v0.211.0 // indirect
	google.golang.org/genproto v0.0.0-20241021214115-324edc3d5d38 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20241118233622-e639e219e697 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241206012308-a4fef0638583 // indirect
	google.golang.org/grpc v1.67.1 // indirect
	google.golang.org/protobuf v1.35.2 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	howett.net/plist v1.0.0 // indirect
	istio.io/api v1.22.3 // indirect
	istio.io/client-go v1.22.3 // indirect
	k8s.io/api v0.30.3 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/kube-openapi v0.0.0-20240730131305-7a9a4e85957e // indirect
	k8s.io/utils v0.0.0-20240711033017-18e509b52bc8 // indirect
	sigs.k8s.io/controller-runtime v0.18.4 // indirect
	sigs.k8s.io/gateway-api v1.1.0 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
)
