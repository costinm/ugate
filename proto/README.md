# Protos defined and used in the repo

Adopting the style of 'buf.build', and keeping the protos dependency-free (except the proto itself)


## BSR

auto-generated code:
  go.buf.build/TEMPLATE_OWNER/TEMPLATE_NAME/MODULE_OWNER/MODULE_NAME

Example: 
"go.buf.build/grpc/go/googleapis/googleapis/google/storage/v1"

## Usage

grpcurl -protoset <(buf build -o -) ...


## Imported packages

- proto - Istio test echo is using the 'proto' package - preserved for compatibility with the test infra. 
- private ca
- meshca
- istio ca
- grpc/grpc-proto - except tls/provider/meshca
- simplified version of envoy
- konectivity from kde
