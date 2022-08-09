// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        v3.14.0
// source: spire/api/types/status.proto

package types

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

type PermissionDeniedDetails_Reason int32

const (
	// Reason unknown.
	PermissionDeniedDetails_UNKNOWN PermissionDeniedDetails_Reason = 0
	// Agent identity has expired.
	PermissionDeniedDetails_AGENT_EXPIRED PermissionDeniedDetails_Reason = 1
	// Identity is not an attested agent.
	PermissionDeniedDetails_AGENT_NOT_ATTESTED PermissionDeniedDetails_Reason = 2
	// Identity is not the active agent identity.
	PermissionDeniedDetails_AGENT_NOT_ACTIVE PermissionDeniedDetails_Reason = 3
	// Agent has been banned.
	PermissionDeniedDetails_AGENT_BANNED PermissionDeniedDetails_Reason = 4
)

// Enum value maps for PermissionDeniedDetails_Reason.
var (
	PermissionDeniedDetails_Reason_name = map[int32]string{
		0: "UNKNOWN",
		1: "AGENT_EXPIRED",
		2: "AGENT_NOT_ATTESTED",
		3: "AGENT_NOT_ACTIVE",
		4: "AGENT_BANNED",
	}
	PermissionDeniedDetails_Reason_value = map[string]int32{
		"UNKNOWN":            0,
		"AGENT_EXPIRED":      1,
		"AGENT_NOT_ATTESTED": 2,
		"AGENT_NOT_ACTIVE":   3,
		"AGENT_BANNED":       4,
	}
)

func (x PermissionDeniedDetails_Reason) Enum() *PermissionDeniedDetails_Reason {
	p := new(PermissionDeniedDetails_Reason)
	*p = x
	return p
}

func (x PermissionDeniedDetails_Reason) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (PermissionDeniedDetails_Reason) Descriptor() protoreflect.EnumDescriptor {
	return file_spire_api_types_status_proto_enumTypes[0].Descriptor()
}

func (PermissionDeniedDetails_Reason) Type() protoreflect.EnumType {
	return &file_spire_api_types_status_proto_enumTypes[0]
}

func (x PermissionDeniedDetails_Reason) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use PermissionDeniedDetails_Reason.Descriptor instead.
func (PermissionDeniedDetails_Reason) EnumDescriptor() ([]byte, []int) {
	return file_spire_api_types_status_proto_rawDescGZIP(), []int{1, 0}
}

type Status struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// A status code, which should be an enum value of google.rpc.Code.
	Code int32 `protobuf:"varint,1,opt,name=code,proto3" json:"code,omitempty"`
	// A developer-facing error message.
	Message string `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
}

func (x *Status) Reset() {
	*x = Status{}
	if protoimpl.UnsafeEnabled {
		mi := &file_spire_api_types_status_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Status) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Status) ProtoMessage() {}

func (x *Status) ProtoReflect() protoreflect.Message {
	mi := &file_spire_api_types_status_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Status.ProtoReflect.Descriptor instead.
func (*Status) Descriptor() ([]byte, []int) {
	return file_spire_api_types_status_proto_rawDescGZIP(), []int{0}
}

func (x *Status) GetCode() int32 {
	if x != nil {
		return x.Code
	}
	return 0
}

func (x *Status) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

type PermissionDeniedDetails struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Reason PermissionDeniedDetails_Reason `protobuf:"varint,1,opt,name=reason,proto3,enum=spire.api.types.PermissionDeniedDetails_Reason" json:"reason,omitempty"`
}

func (x *PermissionDeniedDetails) Reset() {
	*x = PermissionDeniedDetails{}
	if protoimpl.UnsafeEnabled {
		mi := &file_spire_api_types_status_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PermissionDeniedDetails) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PermissionDeniedDetails) ProtoMessage() {}

func (x *PermissionDeniedDetails) ProtoReflect() protoreflect.Message {
	mi := &file_spire_api_types_status_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PermissionDeniedDetails.ProtoReflect.Descriptor instead.
func (*PermissionDeniedDetails) Descriptor() ([]byte, []int) {
	return file_spire_api_types_status_proto_rawDescGZIP(), []int{1}
}

func (x *PermissionDeniedDetails) GetReason() PermissionDeniedDetails_Reason {
	if x != nil {
		return x.Reason
	}
	return PermissionDeniedDetails_UNKNOWN
}

var File_spire_api_types_status_proto protoreflect.FileDescriptor

var file_spire_api_types_status_proto_rawDesc = []byte{
	0x0a, 0x1c, 0x73, 0x70, 0x69, 0x72, 0x65, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x74, 0x79, 0x70, 0x65,
	0x73, 0x2f, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0f,
	0x73, 0x70, 0x69, 0x72, 0x65, 0x2e, 0x61, 0x70, 0x69, 0x2e, 0x74, 0x79, 0x70, 0x65, 0x73, 0x22,
	0x36, 0x0a, 0x06, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x12, 0x0a, 0x04, 0x63, 0x6f, 0x64,
	0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x52, 0x04, 0x63, 0x6f, 0x64, 0x65, 0x12, 0x18, 0x0a,
	0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07,
	0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x22, 0xcc, 0x01, 0x0a, 0x17, 0x50, 0x65, 0x72, 0x6d,
	0x69, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x44, 0x65, 0x6e, 0x69, 0x65, 0x64, 0x44, 0x65, 0x74, 0x61,
	0x69, 0x6c, 0x73, 0x12, 0x47, 0x0a, 0x06, 0x72, 0x65, 0x61, 0x73, 0x6f, 0x6e, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x0e, 0x32, 0x2f, 0x2e, 0x73, 0x70, 0x69, 0x72, 0x65, 0x2e, 0x61, 0x70, 0x69, 0x2e,
	0x74, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x50, 0x65, 0x72, 0x6d, 0x69, 0x73, 0x73, 0x69, 0x6f, 0x6e,
	0x44, 0x65, 0x6e, 0x69, 0x65, 0x64, 0x44, 0x65, 0x74, 0x61, 0x69, 0x6c, 0x73, 0x2e, 0x52, 0x65,
	0x61, 0x73, 0x6f, 0x6e, 0x52, 0x06, 0x72, 0x65, 0x61, 0x73, 0x6f, 0x6e, 0x22, 0x68, 0x0a, 0x06,
	0x52, 0x65, 0x61, 0x73, 0x6f, 0x6e, 0x12, 0x0b, 0x0a, 0x07, 0x55, 0x4e, 0x4b, 0x4e, 0x4f, 0x57,
	0x4e, 0x10, 0x00, 0x12, 0x11, 0x0a, 0x0d, 0x41, 0x47, 0x45, 0x4e, 0x54, 0x5f, 0x45, 0x58, 0x50,
	0x49, 0x52, 0x45, 0x44, 0x10, 0x01, 0x12, 0x16, 0x0a, 0x12, 0x41, 0x47, 0x45, 0x4e, 0x54, 0x5f,
	0x4e, 0x4f, 0x54, 0x5f, 0x41, 0x54, 0x54, 0x45, 0x53, 0x54, 0x45, 0x44, 0x10, 0x02, 0x12, 0x14,
	0x0a, 0x10, 0x41, 0x47, 0x45, 0x4e, 0x54, 0x5f, 0x4e, 0x4f, 0x54, 0x5f, 0x41, 0x43, 0x54, 0x49,
	0x56, 0x45, 0x10, 0x03, 0x12, 0x10, 0x0a, 0x0c, 0x41, 0x47, 0x45, 0x4e, 0x54, 0x5f, 0x42, 0x41,
	0x4e, 0x4e, 0x45, 0x44, 0x10, 0x04, 0x42, 0x37, 0x5a, 0x35, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62,
	0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x73, 0x70, 0x69, 0x66, 0x66, 0x65, 0x2f, 0x73, 0x70, 0x69, 0x72,
	0x65, 0x2d, 0x61, 0x70, 0x69, 0x2d, 0x73, 0x64, 0x6b, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f,
	0x73, 0x70, 0x69, 0x72, 0x65, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x74, 0x79, 0x70, 0x65, 0x73, 0x62,
	0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_spire_api_types_status_proto_rawDescOnce sync.Once
	file_spire_api_types_status_proto_rawDescData = file_spire_api_types_status_proto_rawDesc
)

func file_spire_api_types_status_proto_rawDescGZIP() []byte {
	file_spire_api_types_status_proto_rawDescOnce.Do(func() {
		file_spire_api_types_status_proto_rawDescData = protoimpl.X.CompressGZIP(file_spire_api_types_status_proto_rawDescData)
	})
	return file_spire_api_types_status_proto_rawDescData
}

var file_spire_api_types_status_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_spire_api_types_status_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_spire_api_types_status_proto_goTypes = []interface{}{
	(PermissionDeniedDetails_Reason)(0), // 0: spire.api.types.PermissionDeniedDetails.Reason
	(*Status)(nil),                      // 1: spire.api.types.Status
	(*PermissionDeniedDetails)(nil),     // 2: spire.api.types.PermissionDeniedDetails
}
var file_spire_api_types_status_proto_depIdxs = []int32{
	0, // 0: spire.api.types.PermissionDeniedDetails.reason:type_name -> spire.api.types.PermissionDeniedDetails.Reason
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_spire_api_types_status_proto_init() }
func file_spire_api_types_status_proto_init() {
	if File_spire_api_types_status_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_spire_api_types_status_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Status); i {
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
		file_spire_api_types_status_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PermissionDeniedDetails); i {
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
			RawDescriptor: file_spire_api_types_status_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_spire_api_types_status_proto_goTypes,
		DependencyIndexes: file_spire_api_types_status_proto_depIdxs,
		EnumInfos:         file_spire_api_types_status_proto_enumTypes,
		MessageInfos:      file_spire_api_types_status_proto_msgTypes,
	}.Build()
	File_spire_api_types_status_proto = out.File
	file_spire_api_types_status_proto_rawDesc = nil
	file_spire_api_types_status_proto_goTypes = nil
	file_spire_api_types_status_proto_depIdxs = nil
}
