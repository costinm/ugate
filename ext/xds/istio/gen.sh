#!/bin/bash

T=$GOPATH
S=$T/src/github.com/costinm/dmesh/dm/istio
POUT=$T/src/github.com/costinm/dmesh/dm/istio
I=$T/src:.
export PATH=$GOPATH/bin:$PATH

M="google/protobuf/any.proto=github.com/gogo/protobuf/types"
M="$M,google/protobuf/duration.proto=github.com/gogo/protobuf/types"

function getgogo() {
    go get github.com/gogo/protobuf/proto
    go get github.com/gogo/protobuf/jsonpb
    go get github.com/gogo/protobuf/gogoproto
    go get github.com/gogo/protobuf/protoc-gen-gogo
    go get github.com/gogo/protobuf/protoc-gen-gogofast
    go get github.com/gogo/protobuf/protoc-gen-gogoslick
}

getgogo

SRC=""
for i in *.proto ; do
    SRC="$SRC $S/$i"
done

protoc -I$I --gogo_out=plugins=grpc,$M:$T/src $SRC

#protoc -I$I --gogofast_out=plugins=grpc,$M:$POUT/fast $SRC

#protoc -I$I --gogoslick_out=plugins=grpc,$M:$POUT/slick $SRC


