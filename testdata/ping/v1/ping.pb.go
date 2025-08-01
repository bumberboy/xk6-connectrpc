// Copyright 2024 The k6 Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        (unknown)
// source: ping/v1/ping.proto

// The k6.connectrpc.ping.v1 package contains a test service for the k6 connectrpc extension

package pingv1

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type PingRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Number        int64                  `protobuf:"varint,1,opt,name=number,proto3" json:"number,omitempty"`
	Text          string                 `protobuf:"bytes,2,opt,name=text,proto3" json:"text,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *PingRequest) Reset() {
	*x = PingRequest{}
	mi := &file_ping_v1_ping_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *PingRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PingRequest) ProtoMessage() {}

func (x *PingRequest) ProtoReflect() protoreflect.Message {
	mi := &file_ping_v1_ping_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PingRequest.ProtoReflect.Descriptor instead.
func (*PingRequest) Descriptor() ([]byte, []int) {
	return file_ping_v1_ping_proto_rawDescGZIP(), []int{0}
}

func (x *PingRequest) GetNumber() int64 {
	if x != nil {
		return x.Number
	}
	return 0
}

func (x *PingRequest) GetText() string {
	if x != nil {
		return x.Text
	}
	return ""
}

type PingResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Number        int64                  `protobuf:"varint,1,opt,name=number,proto3" json:"number,omitempty"`
	Text          string                 `protobuf:"bytes,2,opt,name=text,proto3" json:"text,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *PingResponse) Reset() {
	*x = PingResponse{}
	mi := &file_ping_v1_ping_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *PingResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PingResponse) ProtoMessage() {}

func (x *PingResponse) ProtoReflect() protoreflect.Message {
	mi := &file_ping_v1_ping_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PingResponse.ProtoReflect.Descriptor instead.
func (*PingResponse) Descriptor() ([]byte, []int) {
	return file_ping_v1_ping_proto_rawDescGZIP(), []int{1}
}

func (x *PingResponse) GetNumber() int64 {
	if x != nil {
		return x.Number
	}
	return 0
}

func (x *PingResponse) GetText() string {
	if x != nil {
		return x.Text
	}
	return ""
}

type FailRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Code          int32                  `protobuf:"varint,1,opt,name=code,proto3" json:"code,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *FailRequest) Reset() {
	*x = FailRequest{}
	mi := &file_ping_v1_ping_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *FailRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FailRequest) ProtoMessage() {}

func (x *FailRequest) ProtoReflect() protoreflect.Message {
	mi := &file_ping_v1_ping_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FailRequest.ProtoReflect.Descriptor instead.
func (*FailRequest) Descriptor() ([]byte, []int) {
	return file_ping_v1_ping_proto_rawDescGZIP(), []int{2}
}

func (x *FailRequest) GetCode() int32 {
	if x != nil {
		return x.Code
	}
	return 0
}

type FailResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *FailResponse) Reset() {
	*x = FailResponse{}
	mi := &file_ping_v1_ping_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *FailResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FailResponse) ProtoMessage() {}

func (x *FailResponse) ProtoReflect() protoreflect.Message {
	mi := &file_ping_v1_ping_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FailResponse.ProtoReflect.Descriptor instead.
func (*FailResponse) Descriptor() ([]byte, []int) {
	return file_ping_v1_ping_proto_rawDescGZIP(), []int{3}
}

type SumRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Number        int64                  `protobuf:"varint,1,opt,name=number,proto3" json:"number,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SumRequest) Reset() {
	*x = SumRequest{}
	mi := &file_ping_v1_ping_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SumRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SumRequest) ProtoMessage() {}

func (x *SumRequest) ProtoReflect() protoreflect.Message {
	mi := &file_ping_v1_ping_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SumRequest.ProtoReflect.Descriptor instead.
func (*SumRequest) Descriptor() ([]byte, []int) {
	return file_ping_v1_ping_proto_rawDescGZIP(), []int{4}
}

func (x *SumRequest) GetNumber() int64 {
	if x != nil {
		return x.Number
	}
	return 0
}

type SumResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Sum           int64                  `protobuf:"varint,1,opt,name=sum,proto3" json:"sum,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SumResponse) Reset() {
	*x = SumResponse{}
	mi := &file_ping_v1_ping_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SumResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SumResponse) ProtoMessage() {}

func (x *SumResponse) ProtoReflect() protoreflect.Message {
	mi := &file_ping_v1_ping_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SumResponse.ProtoReflect.Descriptor instead.
func (*SumResponse) Descriptor() ([]byte, []int) {
	return file_ping_v1_ping_proto_rawDescGZIP(), []int{5}
}

func (x *SumResponse) GetSum() int64 {
	if x != nil {
		return x.Sum
	}
	return 0
}

type CountUpRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Number        int64                  `protobuf:"varint,1,opt,name=number,proto3" json:"number,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CountUpRequest) Reset() {
	*x = CountUpRequest{}
	mi := &file_ping_v1_ping_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CountUpRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CountUpRequest) ProtoMessage() {}

func (x *CountUpRequest) ProtoReflect() protoreflect.Message {
	mi := &file_ping_v1_ping_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CountUpRequest.ProtoReflect.Descriptor instead.
func (*CountUpRequest) Descriptor() ([]byte, []int) {
	return file_ping_v1_ping_proto_rawDescGZIP(), []int{6}
}

func (x *CountUpRequest) GetNumber() int64 {
	if x != nil {
		return x.Number
	}
	return 0
}

type CountUpResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Number        int64                  `protobuf:"varint,1,opt,name=number,proto3" json:"number,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CountUpResponse) Reset() {
	*x = CountUpResponse{}
	mi := &file_ping_v1_ping_proto_msgTypes[7]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CountUpResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CountUpResponse) ProtoMessage() {}

func (x *CountUpResponse) ProtoReflect() protoreflect.Message {
	mi := &file_ping_v1_ping_proto_msgTypes[7]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CountUpResponse.ProtoReflect.Descriptor instead.
func (*CountUpResponse) Descriptor() ([]byte, []int) {
	return file_ping_v1_ping_proto_rawDescGZIP(), []int{7}
}

func (x *CountUpResponse) GetNumber() int64 {
	if x != nil {
		return x.Number
	}
	return 0
}

type CumSumRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Number        int64                  `protobuf:"varint,1,opt,name=number,proto3" json:"number,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CumSumRequest) Reset() {
	*x = CumSumRequest{}
	mi := &file_ping_v1_ping_proto_msgTypes[8]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CumSumRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CumSumRequest) ProtoMessage() {}

func (x *CumSumRequest) ProtoReflect() protoreflect.Message {
	mi := &file_ping_v1_ping_proto_msgTypes[8]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CumSumRequest.ProtoReflect.Descriptor instead.
func (*CumSumRequest) Descriptor() ([]byte, []int) {
	return file_ping_v1_ping_proto_rawDescGZIP(), []int{8}
}

func (x *CumSumRequest) GetNumber() int64 {
	if x != nil {
		return x.Number
	}
	return 0
}

type CumSumResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Sum           int64                  `protobuf:"varint,1,opt,name=sum,proto3" json:"sum,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CumSumResponse) Reset() {
	*x = CumSumResponse{}
	mi := &file_ping_v1_ping_proto_msgTypes[9]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CumSumResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CumSumResponse) ProtoMessage() {}

func (x *CumSumResponse) ProtoReflect() protoreflect.Message {
	mi := &file_ping_v1_ping_proto_msgTypes[9]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CumSumResponse.ProtoReflect.Descriptor instead.
func (*CumSumResponse) Descriptor() ([]byte, []int) {
	return file_ping_v1_ping_proto_rawDescGZIP(), []int{9}
}

func (x *CumSumResponse) GetSum() int64 {
	if x != nil {
		return x.Sum
	}
	return 0
}

var File_ping_v1_ping_proto protoreflect.FileDescriptor

const file_ping_v1_ping_proto_rawDesc = "" +
	"\n" +
	"\x12ping/v1/ping.proto\x12\x15k6.connectrpc.ping.v1\"9\n" +
	"\vPingRequest\x12\x16\n" +
	"\x06number\x18\x01 \x01(\x03R\x06number\x12\x12\n" +
	"\x04text\x18\x02 \x01(\tR\x04text\":\n" +
	"\fPingResponse\x12\x16\n" +
	"\x06number\x18\x01 \x01(\x03R\x06number\x12\x12\n" +
	"\x04text\x18\x02 \x01(\tR\x04text\"!\n" +
	"\vFailRequest\x12\x12\n" +
	"\x04code\x18\x01 \x01(\x05R\x04code\"\x0e\n" +
	"\fFailResponse\"$\n" +
	"\n" +
	"SumRequest\x12\x16\n" +
	"\x06number\x18\x01 \x01(\x03R\x06number\"\x1f\n" +
	"\vSumResponse\x12\x10\n" +
	"\x03sum\x18\x01 \x01(\x03R\x03sum\"(\n" +
	"\x0eCountUpRequest\x12\x16\n" +
	"\x06number\x18\x01 \x01(\x03R\x06number\")\n" +
	"\x0fCountUpResponse\x12\x16\n" +
	"\x06number\x18\x01 \x01(\x03R\x06number\"'\n" +
	"\rCumSumRequest\x12\x16\n" +
	"\x06number\x18\x01 \x01(\x03R\x06number\"\"\n" +
	"\x0eCumSumResponse\x12\x10\n" +
	"\x03sum\x18\x01 \x01(\x03R\x03sum2\xc3\x03\n" +
	"\vPingService\x12T\n" +
	"\x04Ping\x12\".k6.connectrpc.ping.v1.PingRequest\x1a#.k6.connectrpc.ping.v1.PingResponse\"\x03\x90\x02\x01\x12Q\n" +
	"\x04Fail\x12\".k6.connectrpc.ping.v1.FailRequest\x1a#.k6.connectrpc.ping.v1.FailResponse\"\x00\x12P\n" +
	"\x03Sum\x12!.k6.connectrpc.ping.v1.SumRequest\x1a\".k6.connectrpc.ping.v1.SumResponse\"\x00(\x01\x12\\\n" +
	"\aCountUp\x12%.k6.connectrpc.ping.v1.CountUpRequest\x1a&.k6.connectrpc.ping.v1.CountUpResponse\"\x000\x01\x12[\n" +
	"\x06CumSum\x12$.k6.connectrpc.ping.v1.CumSumRequest\x1a%.k6.connectrpc.ping.v1.CumSumResponse\"\x00(\x010\x01B\xda\x01\n" +
	"\x19com.k6.connectrpc.ping.v1B\tPingProtoP\x01Z;github.com/bumberboy/xk6-connectrpc/testdata/ping/v1;pingv1\xa2\x02\x03KCP\xaa\x02\x15K6.Connectrpc.Ping.V1\xca\x02\x15K6\\Connectrpc\\Ping\\V1\xe2\x02!K6\\Connectrpc\\Ping\\V1\\GPBMetadata\xea\x02\x18K6::Connectrpc::Ping::V1b\x06proto3"

var (
	file_ping_v1_ping_proto_rawDescOnce sync.Once
	file_ping_v1_ping_proto_rawDescData []byte
)

func file_ping_v1_ping_proto_rawDescGZIP() []byte {
	file_ping_v1_ping_proto_rawDescOnce.Do(func() {
		file_ping_v1_ping_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_ping_v1_ping_proto_rawDesc), len(file_ping_v1_ping_proto_rawDesc)))
	})
	return file_ping_v1_ping_proto_rawDescData
}

var file_ping_v1_ping_proto_msgTypes = make([]protoimpl.MessageInfo, 10)
var file_ping_v1_ping_proto_goTypes = []any{
	(*PingRequest)(nil),     // 0: k6.connectrpc.ping.v1.PingRequest
	(*PingResponse)(nil),    // 1: k6.connectrpc.ping.v1.PingResponse
	(*FailRequest)(nil),     // 2: k6.connectrpc.ping.v1.FailRequest
	(*FailResponse)(nil),    // 3: k6.connectrpc.ping.v1.FailResponse
	(*SumRequest)(nil),      // 4: k6.connectrpc.ping.v1.SumRequest
	(*SumResponse)(nil),     // 5: k6.connectrpc.ping.v1.SumResponse
	(*CountUpRequest)(nil),  // 6: k6.connectrpc.ping.v1.CountUpRequest
	(*CountUpResponse)(nil), // 7: k6.connectrpc.ping.v1.CountUpResponse
	(*CumSumRequest)(nil),   // 8: k6.connectrpc.ping.v1.CumSumRequest
	(*CumSumResponse)(nil),  // 9: k6.connectrpc.ping.v1.CumSumResponse
}
var file_ping_v1_ping_proto_depIdxs = []int32{
	0, // 0: k6.connectrpc.ping.v1.PingService.Ping:input_type -> k6.connectrpc.ping.v1.PingRequest
	2, // 1: k6.connectrpc.ping.v1.PingService.Fail:input_type -> k6.connectrpc.ping.v1.FailRequest
	4, // 2: k6.connectrpc.ping.v1.PingService.Sum:input_type -> k6.connectrpc.ping.v1.SumRequest
	6, // 3: k6.connectrpc.ping.v1.PingService.CountUp:input_type -> k6.connectrpc.ping.v1.CountUpRequest
	8, // 4: k6.connectrpc.ping.v1.PingService.CumSum:input_type -> k6.connectrpc.ping.v1.CumSumRequest
	1, // 5: k6.connectrpc.ping.v1.PingService.Ping:output_type -> k6.connectrpc.ping.v1.PingResponse
	3, // 6: k6.connectrpc.ping.v1.PingService.Fail:output_type -> k6.connectrpc.ping.v1.FailResponse
	5, // 7: k6.connectrpc.ping.v1.PingService.Sum:output_type -> k6.connectrpc.ping.v1.SumResponse
	7, // 8: k6.connectrpc.ping.v1.PingService.CountUp:output_type -> k6.connectrpc.ping.v1.CountUpResponse
	9, // 9: k6.connectrpc.ping.v1.PingService.CumSum:output_type -> k6.connectrpc.ping.v1.CumSumResponse
	5, // [5:10] is the sub-list for method output_type
	0, // [0:5] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_ping_v1_ping_proto_init() }
func file_ping_v1_ping_proto_init() {
	if File_ping_v1_ping_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_ping_v1_ping_proto_rawDesc), len(file_ping_v1_ping_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   10,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_ping_v1_ping_proto_goTypes,
		DependencyIndexes: file_ping_v1_ping_proto_depIdxs,
		MessageInfos:      file_ping_v1_ping_proto_msgTypes,
	}.Build()
	File_ping_v1_ping_proto = out.File
	file_ping_v1_ping_proto_goTypes = nil
	file_ping_v1_ping_proto_depIdxs = nil
}
