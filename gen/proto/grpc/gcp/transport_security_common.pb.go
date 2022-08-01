// Copyright 2018 The gRPC Authors
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

// The canonical version of this proto can be found at
// https://github.com/grpc/grpc-proto/blob/master/grpc/gcp/transport_security_common.proto

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.0
// 	protoc        (unknown)
// source: grpc/gcp/transport_security_common.proto

package grpc_gcp

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// The security level of the created channel. The list is sorted in increasing
// level of security. This order must always be maintained.
type SecurityLevel int32

const (
	SecurityLevel_SECURITY_NONE         SecurityLevel = 0
	SecurityLevel_INTEGRITY_ONLY        SecurityLevel = 1
	SecurityLevel_INTEGRITY_AND_PRIVACY SecurityLevel = 2
)

// Enum value maps for SecurityLevel.
var (
	SecurityLevel_name = map[int32]string{
		0: "SECURITY_NONE",
		1: "INTEGRITY_ONLY",
		2: "INTEGRITY_AND_PRIVACY",
	}
	SecurityLevel_value = map[string]int32{
		"SECURITY_NONE":         0,
		"INTEGRITY_ONLY":        1,
		"INTEGRITY_AND_PRIVACY": 2,
	}
)

func (x SecurityLevel) Enum() *SecurityLevel {
	p := new(SecurityLevel)
	*p = x
	return p
}

func (x SecurityLevel) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (SecurityLevel) Descriptor() protoreflect.EnumDescriptor {
	return file_grpc_gcp_transport_security_common_proto_enumTypes[0].Descriptor()
}

func (SecurityLevel) Type() protoreflect.EnumType {
	return &file_grpc_gcp_transport_security_common_proto_enumTypes[0]
}

func (x SecurityLevel) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use SecurityLevel.Descriptor instead.
func (SecurityLevel) EnumDescriptor() ([]byte, []int) {
	return file_grpc_gcp_transport_security_common_proto_rawDescGZIP(), []int{0}
}

// Max and min supported RPC protocol versions.
type RpcProtocolVersions struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Maximum supported RPC version.
	MaxRpcVersion *RpcProtocolVersions_Version `protobuf:"bytes,1,opt,name=max_rpc_version,json=maxRpcVersion,proto3" json:"max_rpc_version,omitempty"`
	// Minimum supported RPC version.
	MinRpcVersion *RpcProtocolVersions_Version `protobuf:"bytes,2,opt,name=min_rpc_version,json=minRpcVersion,proto3" json:"min_rpc_version,omitempty"`
}

func (x *RpcProtocolVersions) Reset() {
	*x = RpcProtocolVersions{}
	if protoimpl.UnsafeEnabled {
		mi := &file_grpc_gcp_transport_security_common_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RpcProtocolVersions) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RpcProtocolVersions) ProtoMessage() {}

func (x *RpcProtocolVersions) ProtoReflect() protoreflect.Message {
	mi := &file_grpc_gcp_transport_security_common_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RpcProtocolVersions.ProtoReflect.Descriptor instead.
func (*RpcProtocolVersions) Descriptor() ([]byte, []int) {
	return file_grpc_gcp_transport_security_common_proto_rawDescGZIP(), []int{0}
}

func (x *RpcProtocolVersions) GetMaxRpcVersion() *RpcProtocolVersions_Version {
	if x != nil {
		return x.MaxRpcVersion
	}
	return nil
}

func (x *RpcProtocolVersions) GetMinRpcVersion() *RpcProtocolVersions_Version {
	if x != nil {
		return x.MinRpcVersion
	}
	return nil
}

// RPC version contains a major version and a minor version.
type RpcProtocolVersions_Version struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Major uint32 `protobuf:"varint,1,opt,name=major,proto3" json:"major,omitempty"`
	Minor uint32 `protobuf:"varint,2,opt,name=minor,proto3" json:"minor,omitempty"`
}

func (x *RpcProtocolVersions_Version) Reset() {
	*x = RpcProtocolVersions_Version{}
	if protoimpl.UnsafeEnabled {
		mi := &file_grpc_gcp_transport_security_common_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RpcProtocolVersions_Version) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RpcProtocolVersions_Version) ProtoMessage() {}

func (x *RpcProtocolVersions_Version) ProtoReflect() protoreflect.Message {
	mi := &file_grpc_gcp_transport_security_common_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RpcProtocolVersions_Version.ProtoReflect.Descriptor instead.
func (*RpcProtocolVersions_Version) Descriptor() ([]byte, []int) {
	return file_grpc_gcp_transport_security_common_proto_rawDescGZIP(), []int{0, 0}
}

func (x *RpcProtocolVersions_Version) GetMajor() uint32 {
	if x != nil {
		return x.Major
	}
	return 0
}

func (x *RpcProtocolVersions_Version) GetMinor() uint32 {
	if x != nil {
		return x.Minor
	}
	return 0
}

var File_grpc_gcp_transport_security_common_proto protoreflect.FileDescriptor

var file_grpc_gcp_transport_security_common_proto_rawDesc = []byte{
	0x0a, 0x28, 0x67, 0x72, 0x70, 0x63, 0x2f, 0x67, 0x63, 0x70, 0x2f, 0x74, 0x72, 0x61, 0x6e, 0x73,
	0x70, 0x6f, 0x72, 0x74, 0x5f, 0x73, 0x65, 0x63, 0x75, 0x72, 0x69, 0x74, 0x79, 0x5f, 0x63, 0x6f,
	0x6d, 0x6d, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x08, 0x67, 0x72, 0x70, 0x63,
	0x2e, 0x67, 0x63, 0x70, 0x22, 0xea, 0x01, 0x0a, 0x13, 0x52, 0x70, 0x63, 0x50, 0x72, 0x6f, 0x74,
	0x6f, 0x63, 0x6f, 0x6c, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x4d, 0x0a, 0x0f,
	0x6d, 0x61, 0x78, 0x5f, 0x72, 0x70, 0x63, 0x5f, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x25, 0x2e, 0x67, 0x72, 0x70, 0x63, 0x2e, 0x67, 0x63, 0x70,
	0x2e, 0x52, 0x70, 0x63, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x56, 0x65, 0x72, 0x73,
	0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x52, 0x0d, 0x6d, 0x61,
	0x78, 0x52, 0x70, 0x63, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x4d, 0x0a, 0x0f, 0x6d,
	0x69, 0x6e, 0x5f, 0x72, 0x70, 0x63, 0x5f, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x25, 0x2e, 0x67, 0x72, 0x70, 0x63, 0x2e, 0x67, 0x63, 0x70, 0x2e,
	0x52, 0x70, 0x63, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x56, 0x65, 0x72, 0x73, 0x69,
	0x6f, 0x6e, 0x73, 0x2e, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x52, 0x0d, 0x6d, 0x69, 0x6e,
	0x52, 0x70, 0x63, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x1a, 0x35, 0x0a, 0x07, 0x56, 0x65,
	0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x14, 0x0a, 0x05, 0x6d, 0x61, 0x6a, 0x6f, 0x72, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x0d, 0x52, 0x05, 0x6d, 0x61, 0x6a, 0x6f, 0x72, 0x12, 0x14, 0x0a, 0x05, 0x6d,
	0x69, 0x6e, 0x6f, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x05, 0x6d, 0x69, 0x6e, 0x6f,
	0x72, 0x2a, 0x51, 0x0a, 0x0d, 0x53, 0x65, 0x63, 0x75, 0x72, 0x69, 0x74, 0x79, 0x4c, 0x65, 0x76,
	0x65, 0x6c, 0x12, 0x11, 0x0a, 0x0d, 0x53, 0x45, 0x43, 0x55, 0x52, 0x49, 0x54, 0x59, 0x5f, 0x4e,
	0x4f, 0x4e, 0x45, 0x10, 0x00, 0x12, 0x12, 0x0a, 0x0e, 0x49, 0x4e, 0x54, 0x45, 0x47, 0x52, 0x49,
	0x54, 0x59, 0x5f, 0x4f, 0x4e, 0x4c, 0x59, 0x10, 0x01, 0x12, 0x19, 0x0a, 0x15, 0x49, 0x4e, 0x54,
	0x45, 0x47, 0x52, 0x49, 0x54, 0x59, 0x5f, 0x41, 0x4e, 0x44, 0x5f, 0x50, 0x52, 0x49, 0x56, 0x41,
	0x43, 0x59, 0x10, 0x02, 0x42, 0x78, 0x0a, 0x15, 0x69, 0x6f, 0x2e, 0x67, 0x72, 0x70, 0x63, 0x2e,
	0x61, 0x6c, 0x74, 0x73, 0x2e, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x42, 0x1c, 0x54,
	0x72, 0x61, 0x6e, 0x73, 0x70, 0x6f, 0x72, 0x74, 0x53, 0x65, 0x63, 0x75, 0x72, 0x69, 0x74, 0x79,
	0x43, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x3f, 0x67,
	0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x67, 0x6f, 0x6c, 0x61, 0x6e, 0x67, 0x2e, 0x6f, 0x72, 0x67,
	0x2f, 0x67, 0x72, 0x70, 0x63, 0x2f, 0x63, 0x72, 0x65, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x61, 0x6c,
	0x73, 0x2f, 0x61, 0x6c, 0x74, 0x73, 0x2f, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x67, 0x72, 0x70, 0x63, 0x5f, 0x67, 0x63, 0x70, 0x62, 0x06,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_grpc_gcp_transport_security_common_proto_rawDescOnce sync.Once
	file_grpc_gcp_transport_security_common_proto_rawDescData = file_grpc_gcp_transport_security_common_proto_rawDesc
)

func file_grpc_gcp_transport_security_common_proto_rawDescGZIP() []byte {
	file_grpc_gcp_transport_security_common_proto_rawDescOnce.Do(func() {
		file_grpc_gcp_transport_security_common_proto_rawDescData = protoimpl.X.CompressGZIP(file_grpc_gcp_transport_security_common_proto_rawDescData)
	})
	return file_grpc_gcp_transport_security_common_proto_rawDescData
}

var file_grpc_gcp_transport_security_common_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_grpc_gcp_transport_security_common_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_grpc_gcp_transport_security_common_proto_goTypes = []interface{}{
	(SecurityLevel)(0),                  // 0: grpc.gcp.SecurityLevel
	(*RpcProtocolVersions)(nil),         // 1: grpc.gcp.RpcProtocolVersions
	(*RpcProtocolVersions_Version)(nil), // 2: grpc.gcp.RpcProtocolVersions.Version
}
var file_grpc_gcp_transport_security_common_proto_depIdxs = []int32{
	2, // 0: grpc.gcp.RpcProtocolVersions.max_rpc_version:type_name -> grpc.gcp.RpcProtocolVersions.Version
	2, // 1: grpc.gcp.RpcProtocolVersions.min_rpc_version:type_name -> grpc.gcp.RpcProtocolVersions.Version
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_grpc_gcp_transport_security_common_proto_init() }
func file_grpc_gcp_transport_security_common_proto_init() {
	if File_grpc_gcp_transport_security_common_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_grpc_gcp_transport_security_common_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RpcProtocolVersions); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_grpc_gcp_transport_security_common_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RpcProtocolVersions_Version); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_grpc_gcp_transport_security_common_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_grpc_gcp_transport_security_common_proto_goTypes,
		DependencyIndexes: file_grpc_gcp_transport_security_common_proto_depIdxs,
		EnumInfos:         file_grpc_gcp_transport_security_common_proto_enumTypes,
		MessageInfos:      file_grpc_gcp_transport_security_common_proto_msgTypes,
	}.Build()
	File_grpc_gcp_transport_security_common_proto = out.File
	file_grpc_gcp_transport_security_common_proto_rawDesc = nil
	file_grpc_gcp_transport_security_common_proto_goTypes = nil
	file_grpc_gcp_transport_security_common_proto_depIdxs = nil
}
