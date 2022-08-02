// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.0
// 	protoc        (unknown)
// source: webpush/webpush.proto

package msgs

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

// Common fields to be encoded in the 'data' proto
type MessageData struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Time when the message was sent, according to the sender clock.
	Time int64 `protobuf:"varint,1,opt,name=time,proto3" json:"time,omitempty"`
	// Original ID. If missing, the envelope ID will be used.
	Id string `protobuf:"bytes,2,opt,name=id,proto3" json:"id,omitempty"`
	// Original destination
	To    string            `protobuf:"bytes,3,opt,name=to,proto3" json:"to,omitempty"`
	From  string            `protobuf:"bytes,4,opt,name=from,proto3" json:"from,omitempty"`
	Topic string            `protobuf:"bytes,5,opt,name=topic,proto3" json:"topic,omitempty"`
	Meta  map[string]string `protobuf:"bytes,6,rep,name=meta,proto3" json:"meta,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	Data  []byte            `protobuf:"bytes,7,opt,name=data,proto3" json:"data,omitempty"`
}

func (x *MessageData) Reset() {
	*x = MessageData{}
	if protoimpl.UnsafeEnabled {
		mi := &file_webpush_webpush_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *MessageData) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MessageData) ProtoMessage() {}

func (x *MessageData) ProtoReflect() protoreflect.Message {
	mi := &file_webpush_webpush_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use MessageData.ProtoReflect.Descriptor instead.
func (*MessageData) Descriptor() ([]byte, []int) {
	return file_webpush_webpush_proto_rawDescGZIP(), []int{0}
}

func (x *MessageData) GetTime() int64 {
	if x != nil {
		return x.Time
	}
	return 0
}

func (x *MessageData) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *MessageData) GetTo() string {
	if x != nil {
		return x.To
	}
	return ""
}

func (x *MessageData) GetFrom() string {
	if x != nil {
		return x.From
	}
	return ""
}

func (x *MessageData) GetTopic() string {
	if x != nil {
		return x.Topic
	}
	return ""
}

func (x *MessageData) GetMeta() map[string]string {
	if x != nil {
		return x.Meta
	}
	return nil
}

func (x *MessageData) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

// Message is returned as PUSH PROMISE frames in the spec. The alternative protocol wraps it in
// Any field or other framing.
type WebpushMessage struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Unique in context of the origin
	// For webpush, generated by the original server (from subscription), as Location:
	// Example: https://push.example.net/message/qDIYHNcfAIPP_5ITvURr-d6BGt
	Id string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	// Plaintext = 0
	// aes128gcm = 1
	ContentEncoding int32 `protobuf:"varint,7,opt,name=content_encoding,json=contentEncoding,proto3" json:"content_encoding,omitempty"`
	// If encoding is "1" - aes128gcm
	// Otherwise it is a plaintext message.
	Data []byte `protobuf:"bytes,3,opt,name=data,proto3" json:"data,omitempty"`
	// Message path
	Path []*Via `protobuf:"bytes,6,rep,name=path,proto3" json:"path,omitempty"`
	Ttl  int32  `protobuf:"varint,9,opt,name=ttl,proto3" json:"ttl,omitempty"`
	// Maps to the SubscribeResponse push parameter, returned as Link rel="urn:ietf:params:push"
	// in the push promise.
	Push string `protobuf:"bytes,2,opt,name=push,proto3" json:"push,omitempty"`
	// Identifies the sender - compact form.
	Sender *Vapid `protobuf:"bytes,4,opt,name=sender,proto3" json:"sender,omitempty"`
	// URL or IPv6, extracted from the VAPID of the sender or other
	// form of authentication.
	From string `protobuf:"bytes,5,opt,name=from,proto3" json:"from,omitempty"`
}

func (x *WebpushMessage) Reset() {
	*x = WebpushMessage{}
	if protoimpl.UnsafeEnabled {
		mi := &file_webpush_webpush_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *WebpushMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*WebpushMessage) ProtoMessage() {}

func (x *WebpushMessage) ProtoReflect() protoreflect.Message {
	mi := &file_webpush_webpush_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use WebpushMessage.ProtoReflect.Descriptor instead.
func (*WebpushMessage) Descriptor() ([]byte, []int) {
	return file_webpush_webpush_proto_rawDescGZIP(), []int{1}
}

func (x *WebpushMessage) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *WebpushMessage) GetContentEncoding() int32 {
	if x != nil {
		return x.ContentEncoding
	}
	return 0
}

func (x *WebpushMessage) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

func (x *WebpushMessage) GetPath() []*Via {
	if x != nil {
		return x.Path
	}
	return nil
}

func (x *WebpushMessage) GetTtl() int32 {
	if x != nil {
		return x.Ttl
	}
	return 0
}

func (x *WebpushMessage) GetPush() string {
	if x != nil {
		return x.Push
	}
	return ""
}

func (x *WebpushMessage) GetSender() *Vapid {
	if x != nil {
		return x.Sender
	}
	return nil
}

func (x *WebpushMessage) GetFrom() string {
	if x != nil {
		return x.From
	}
	return ""
}

type Via struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Time int64  `protobuf:"varint,1,opt,name=time,proto3" json:"time,omitempty"`
	Vip  string `protobuf:"bytes,2,opt,name=vip,proto3" json:"vip,omitempty"`
}

func (x *Via) Reset() {
	*x = Via{}
	if protoimpl.UnsafeEnabled {
		mi := &file_webpush_webpush_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Via) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Via) ProtoMessage() {}

func (x *Via) ProtoReflect() protoreflect.Message {
	mi := &file_webpush_webpush_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Via.ProtoReflect.Descriptor instead.
func (*Via) Descriptor() ([]byte, []int) {
	return file_webpush_webpush_proto_rawDescGZIP(), []int{2}
}

func (x *Via) GetTime() int64 {
	if x != nil {
		return x.Time
	}
	return 0
}

func (x *Via) GetVip() string {
	if x != nil {
		return x.Vip
	}
	return ""
}

// Vapid is the proto variant of a Webpush JWT.
// This is a more compact representation, without base64 overhead
//
// For HTTP, included in Authorization header:
// Authorization: vapid t=B64url k=B64url
//
// Decoded t is of form: { "typ": "JWT", "alg": "ES256" }.JWT.SIG
//
// { "crv":"P-256",
//   "kty":"EC",
//   "x":"DUfHPKLVFQzVvnCPGyfucbECzPDa7rWbXriLcysAjEc",
//   "y":"F6YK5h4SDYic-dRuU_RCPCfA5aq9ojSwk5Y2EmClBPs" }
type Vapid struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// json payload of VAPID ( without base64 encoding)
	// Can also be a proto message when used over other transports.
	// Verification requires converting back to base64 !
	// Decoded to reduce the binary size
	Data []byte `protobuf:"bytes,7,opt,name=data,proto3" json:"data,omitempty"`
	// Public key of the signer, 64 bytes, EC256, decoded.
	// Included in 'k' parameter for HTTP.
	K []byte `protobuf:"bytes,4,opt,name=k,proto3" json:"k,omitempty"`
	// If empty, it is assumed to be the constant value {typ=JWT,alg=ES256}
	TType []byte `protobuf:"bytes,32,opt,name=t_type,json=tType,proto3" json:"t_type,omitempty"`
	// Decoded
	TSignature []byte `protobuf:"bytes,33,opt,name=t_signature,json=tSignature,proto3" json:"t_signature,omitempty"`
}

func (x *Vapid) Reset() {
	*x = Vapid{}
	if protoimpl.UnsafeEnabled {
		mi := &file_webpush_webpush_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Vapid) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Vapid) ProtoMessage() {}

func (x *Vapid) ProtoReflect() protoreflect.Message {
	mi := &file_webpush_webpush_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Vapid.ProtoReflect.Descriptor instead.
func (*Vapid) Descriptor() ([]byte, []int) {
	return file_webpush_webpush_proto_rawDescGZIP(), []int{3}
}

func (x *Vapid) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

func (x *Vapid) GetK() []byte {
	if x != nil {
		return x.K
	}
	return nil
}

func (x *Vapid) GetTType() []byte {
	if x != nil {
		return x.TType
	}
	return nil
}

func (x *Vapid) GetTSignature() []byte {
	if x != nil {
		return x.TSignature
	}
	return nil
}

type PushRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The value returned in the SubscribeResponse push, without the hostname.
	Push string `protobuf:"bytes,1,opt,name=push,proto3" json:"push,omitempty"`
	Ttl  int32  `protobuf:"varint,2,opt,name=ttl,proto3" json:"ttl,omitempty"`
	// aes128gcm encrypted
	Data    []byte `protobuf:"bytes,3,opt,name=data,proto3" json:"data,omitempty"`
	Urgency string `protobuf:"bytes,4,opt,name=urgency,proto3" json:"urgency,omitempty"`
	// Prefer header indicating delivery receipt request.
	RespondAsync bool   `protobuf:"varint,5,opt,name=respond_async,json=respondAsync,proto3" json:"respond_async,omitempty"`
	Topic        string `protobuf:"bytes,6,opt,name=topic,proto3" json:"topic,omitempty"`
}

func (x *PushRequest) Reset() {
	*x = PushRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_webpush_webpush_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PushRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PushRequest) ProtoMessage() {}

func (x *PushRequest) ProtoReflect() protoreflect.Message {
	mi := &file_webpush_webpush_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PushRequest.ProtoReflect.Descriptor instead.
func (*PushRequest) Descriptor() ([]byte, []int) {
	return file_webpush_webpush_proto_rawDescGZIP(), []int{4}
}

func (x *PushRequest) GetPush() string {
	if x != nil {
		return x.Push
	}
	return ""
}

func (x *PushRequest) GetTtl() int32 {
	if x != nil {
		return x.Ttl
	}
	return 0
}

func (x *PushRequest) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

func (x *PushRequest) GetUrgency() string {
	if x != nil {
		return x.Urgency
	}
	return ""
}

func (x *PushRequest) GetRespondAsync() bool {
	if x != nil {
		return x.RespondAsync
	}
	return false
}

func (x *PushRequest) GetTopic() string {
	if x != nil {
		return x.Topic
	}
	return ""
}

type PushResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	MessageId string `protobuf:"bytes,1,opt,name=message_id,json=messageId,proto3" json:"message_id,omitempty"`
	// If request includes the respond_async parameter.
	//
	PushReceipt string `protobuf:"bytes,2,opt,name=push_receipt,json=pushReceipt,proto3" json:"push_receipt,omitempty"`
}

func (x *PushResponse) Reset() {
	*x = PushResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_webpush_webpush_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PushResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PushResponse) ProtoMessage() {}

func (x *PushResponse) ProtoReflect() protoreflect.Message {
	mi := &file_webpush_webpush_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PushResponse.ProtoReflect.Descriptor instead.
func (*PushResponse) Descriptor() ([]byte, []int) {
	return file_webpush_webpush_proto_rawDescGZIP(), []int{5}
}

func (x *PushResponse) GetMessageId() string {
	if x != nil {
		return x.MessageId
	}
	return ""
}

func (x *PushResponse) GetPushReceipt() string {
	if x != nil {
		return x.PushReceipt
	}
	return ""
}

var File_webpush_webpush_proto protoreflect.FileDescriptor

var file_webpush_webpush_proto_rawDesc = []byte{
	0x0a, 0x15, 0x77, 0x65, 0x62, 0x70, 0x75, 0x73, 0x68, 0x2f, 0x77, 0x65, 0x62, 0x70, 0x75, 0x73,
	0x68, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x07, 0x77, 0x65, 0x62, 0x70, 0x75, 0x73, 0x68,
	0x22, 0xec, 0x01, 0x0a, 0x0b, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x44, 0x61, 0x74, 0x61,
	0x12, 0x12, 0x0a, 0x04, 0x74, 0x69, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x04,
	0x74, 0x69, 0x6d, 0x65, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x02, 0x69, 0x64, 0x12, 0x0e, 0x0a, 0x02, 0x74, 0x6f, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x02, 0x74, 0x6f, 0x12, 0x12, 0x0a, 0x04, 0x66, 0x72, 0x6f, 0x6d, 0x18, 0x04, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x04, 0x66, 0x72, 0x6f, 0x6d, 0x12, 0x14, 0x0a, 0x05, 0x74, 0x6f, 0x70, 0x69,
	0x63, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x74, 0x6f, 0x70, 0x69, 0x63, 0x12, 0x32,
	0x0a, 0x04, 0x6d, 0x65, 0x74, 0x61, 0x18, 0x06, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1e, 0x2e, 0x77,
	0x65, 0x62, 0x70, 0x75, 0x73, 0x68, 0x2e, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x44, 0x61,
	0x74, 0x61, 0x2e, 0x4d, 0x65, 0x74, 0x61, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x04, 0x6d, 0x65,
	0x74, 0x61, 0x12, 0x12, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x07, 0x20, 0x01, 0x28, 0x0c,
	0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x1a, 0x37, 0x0a, 0x09, 0x4d, 0x65, 0x74, 0x61, 0x45, 0x6e,
	0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x22,
	0xe3, 0x01, 0x0a, 0x0e, 0x57, 0x65, 0x62, 0x70, 0x75, 0x73, 0x68, 0x4d, 0x65, 0x73, 0x73, 0x61,
	0x67, 0x65, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02,
	0x69, 0x64, 0x12, 0x29, 0x0a, 0x10, 0x63, 0x6f, 0x6e, 0x74, 0x65, 0x6e, 0x74, 0x5f, 0x65, 0x6e,
	0x63, 0x6f, 0x64, 0x69, 0x6e, 0x67, 0x18, 0x07, 0x20, 0x01, 0x28, 0x05, 0x52, 0x0f, 0x63, 0x6f,
	0x6e, 0x74, 0x65, 0x6e, 0x74, 0x45, 0x6e, 0x63, 0x6f, 0x64, 0x69, 0x6e, 0x67, 0x12, 0x12, 0x0a,
	0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x04, 0x64, 0x61, 0x74,
	0x61, 0x12, 0x20, 0x0a, 0x04, 0x70, 0x61, 0x74, 0x68, 0x18, 0x06, 0x20, 0x03, 0x28, 0x0b, 0x32,
	0x0c, 0x2e, 0x77, 0x65, 0x62, 0x70, 0x75, 0x73, 0x68, 0x2e, 0x56, 0x69, 0x61, 0x52, 0x04, 0x70,
	0x61, 0x74, 0x68, 0x12, 0x10, 0x0a, 0x03, 0x74, 0x74, 0x6c, 0x18, 0x09, 0x20, 0x01, 0x28, 0x05,
	0x52, 0x03, 0x74, 0x74, 0x6c, 0x12, 0x12, 0x0a, 0x04, 0x70, 0x75, 0x73, 0x68, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x04, 0x70, 0x75, 0x73, 0x68, 0x12, 0x26, 0x0a, 0x06, 0x73, 0x65, 0x6e,
	0x64, 0x65, 0x72, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0e, 0x2e, 0x77, 0x65, 0x62, 0x70,
	0x75, 0x73, 0x68, 0x2e, 0x56, 0x61, 0x70, 0x69, 0x64, 0x52, 0x06, 0x73, 0x65, 0x6e, 0x64, 0x65,
	0x72, 0x12, 0x12, 0x0a, 0x04, 0x66, 0x72, 0x6f, 0x6d, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x04, 0x66, 0x72, 0x6f, 0x6d, 0x22, 0x2b, 0x0a, 0x03, 0x56, 0x69, 0x61, 0x12, 0x12, 0x0a, 0x04,
	0x74, 0x69, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x04, 0x74, 0x69, 0x6d, 0x65,
	0x12, 0x10, 0x0a, 0x03, 0x76, 0x69, 0x70, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x76,
	0x69, 0x70, 0x22, 0x61, 0x0a, 0x05, 0x56, 0x61, 0x70, 0x69, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x64,
	0x61, 0x74, 0x61, 0x18, 0x07, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x12,
	0x0c, 0x0a, 0x01, 0x6b, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x01, 0x6b, 0x12, 0x15, 0x0a,
	0x06, 0x74, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x18, 0x20, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x05, 0x74,
	0x54, 0x79, 0x70, 0x65, 0x12, 0x1f, 0x0a, 0x0b, 0x74, 0x5f, 0x73, 0x69, 0x67, 0x6e, 0x61, 0x74,
	0x75, 0x72, 0x65, 0x18, 0x21, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x0a, 0x74, 0x53, 0x69, 0x67, 0x6e,
	0x61, 0x74, 0x75, 0x72, 0x65, 0x22, 0x9c, 0x01, 0x0a, 0x0b, 0x50, 0x75, 0x73, 0x68, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x70, 0x75, 0x73, 0x68, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x04, 0x70, 0x75, 0x73, 0x68, 0x12, 0x10, 0x0a, 0x03, 0x74, 0x74, 0x6c,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x05, 0x52, 0x03, 0x74, 0x74, 0x6c, 0x12, 0x12, 0x0a, 0x04, 0x64,
	0x61, 0x74, 0x61, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x12,
	0x18, 0x0a, 0x07, 0x75, 0x72, 0x67, 0x65, 0x6e, 0x63, 0x79, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x07, 0x75, 0x72, 0x67, 0x65, 0x6e, 0x63, 0x79, 0x12, 0x23, 0x0a, 0x0d, 0x72, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x64, 0x5f, 0x61, 0x73, 0x79, 0x6e, 0x63, 0x18, 0x05, 0x20, 0x01, 0x28, 0x08,
	0x52, 0x0c, 0x72, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x64, 0x41, 0x73, 0x79, 0x6e, 0x63, 0x12, 0x14,
	0x0a, 0x05, 0x74, 0x6f, 0x70, 0x69, 0x63, 0x18, 0x06, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x74,
	0x6f, 0x70, 0x69, 0x63, 0x22, 0x50, 0x0a, 0x0c, 0x50, 0x75, 0x73, 0x68, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x12, 0x1d, 0x0a, 0x0a, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x5f,
	0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67,
	0x65, 0x49, 0x64, 0x12, 0x21, 0x0a, 0x0c, 0x70, 0x75, 0x73, 0x68, 0x5f, 0x72, 0x65, 0x63, 0x65,
	0x69, 0x70, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x70, 0x75, 0x73, 0x68, 0x52,
	0x65, 0x63, 0x65, 0x69, 0x70, 0x74, 0x42, 0x24, 0x5a, 0x22, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62,
	0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x63, 0x6f, 0x73, 0x74, 0x69, 0x6e, 0x6d, 0x2f, 0x77, 0x70, 0x67,
	0x61, 0x74, 0x65, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x6d, 0x73, 0x67, 0x73, 0x62, 0x06, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_webpush_webpush_proto_rawDescOnce sync.Once
	file_webpush_webpush_proto_rawDescData = file_webpush_webpush_proto_rawDesc
)

func file_webpush_webpush_proto_rawDescGZIP() []byte {
	file_webpush_webpush_proto_rawDescOnce.Do(func() {
		file_webpush_webpush_proto_rawDescData = protoimpl.X.CompressGZIP(file_webpush_webpush_proto_rawDescData)
	})
	return file_webpush_webpush_proto_rawDescData
}

var file_webpush_webpush_proto_msgTypes = make([]protoimpl.MessageInfo, 7)
var file_webpush_webpush_proto_goTypes = []interface{}{
	(*MessageData)(nil),    // 0: webpush.MessageData
	(*WebpushMessage)(nil), // 1: webpush.WebpushMessage
	(*Via)(nil),            // 2: webpush.Via
	(*Vapid)(nil),          // 3: webpush.Vapid
	(*PushRequest)(nil),    // 4: webpush.PushRequest
	(*PushResponse)(nil),   // 5: webpush.PushResponse
	nil,                    // 6: webpush.MessageData.MetaEntry
}
var file_webpush_webpush_proto_depIdxs = []int32{
	6, // 0: webpush.MessageData.meta:type_name -> webpush.MessageData.MetaEntry
	2, // 1: webpush.WebpushMessage.path:type_name -> webpush.Via
	3, // 2: webpush.WebpushMessage.sender:type_name -> webpush.Vapid
	3, // [3:3] is the sub-list for method output_type
	3, // [3:3] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_webpush_webpush_proto_init() }
func file_webpush_webpush_proto_init() {
	if File_webpush_webpush_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_webpush_webpush_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*MessageData); i {
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
		file_webpush_webpush_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*WebpushMessage); i {
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
		file_webpush_webpush_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Via); i {
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
		file_webpush_webpush_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Vapid); i {
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
		file_webpush_webpush_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PushRequest); i {
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
		file_webpush_webpush_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PushResponse); i {
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
			RawDescriptor: file_webpush_webpush_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   7,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_webpush_webpush_proto_goTypes,
		DependencyIndexes: file_webpush_webpush_proto_depIdxs,
		MessageInfos:      file_webpush_webpush_proto_msgTypes,
	}.Build()
	File_webpush_webpush_proto = out.File
	file_webpush_webpush_proto_rawDesc = nil
	file_webpush_webpush_proto_goTypes = nil
	file_webpush_webpush_proto_depIdxs = nil
}