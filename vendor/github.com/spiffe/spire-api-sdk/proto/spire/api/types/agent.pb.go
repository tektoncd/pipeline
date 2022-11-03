// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.0
// 	protoc        v3.20.1
// source: spire/api/types/agent.proto

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

type Agent struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Output only. SPIFFE ID of the agent.
	Id *SPIFFEID `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	// Output only. The method by which the agent attested.
	AttestationType string `protobuf:"bytes,2,opt,name=attestation_type,json=attestationType,proto3" json:"attestation_type,omitempty"`
	// Output only. The X509-SVID serial number.
	X509SvidSerialNumber string `protobuf:"bytes,3,opt,name=x509svid_serial_number,json=x509svidSerialNumber,proto3" json:"x509svid_serial_number,omitempty"`
	// Output only. The X509-SVID expiration (seconds since Unix epoch).
	X509SvidExpiresAt int64 `protobuf:"varint,4,opt,name=x509svid_expires_at,json=x509svidExpiresAt,proto3" json:"x509svid_expires_at,omitempty"`
	// Output only. The selectors attributed to the agent during attestation.
	Selectors []*Selector `protobuf:"bytes,5,rep,name=selectors,proto3" json:"selectors,omitempty"`
	// Output only. Whether or not the agent is banned.
	Banned bool `protobuf:"varint,6,opt,name=banned,proto3" json:"banned,omitempty"`
}

func (x *Agent) Reset() {
	*x = Agent{}
	if protoimpl.UnsafeEnabled {
		mi := &file_spire_api_types_agent_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Agent) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Agent) ProtoMessage() {}

func (x *Agent) ProtoReflect() protoreflect.Message {
	mi := &file_spire_api_types_agent_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Agent.ProtoReflect.Descriptor instead.
func (*Agent) Descriptor() ([]byte, []int) {
	return file_spire_api_types_agent_proto_rawDescGZIP(), []int{0}
}

func (x *Agent) GetId() *SPIFFEID {
	if x != nil {
		return x.Id
	}
	return nil
}

func (x *Agent) GetAttestationType() string {
	if x != nil {
		return x.AttestationType
	}
	return ""
}

func (x *Agent) GetX509SvidSerialNumber() string {
	if x != nil {
		return x.X509SvidSerialNumber
	}
	return ""
}

func (x *Agent) GetX509SvidExpiresAt() int64 {
	if x != nil {
		return x.X509SvidExpiresAt
	}
	return 0
}

func (x *Agent) GetSelectors() []*Selector {
	if x != nil {
		return x.Selectors
	}
	return nil
}

func (x *Agent) GetBanned() bool {
	if x != nil {
		return x.Banned
	}
	return false
}

type AgentMask struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// attestation_type field mask
	AttestationType bool `protobuf:"varint,2,opt,name=attestation_type,json=attestationType,proto3" json:"attestation_type,omitempty"`
	// x509svid_serial_number field mask
	X509SvidSerialNumber bool `protobuf:"varint,3,opt,name=x509svid_serial_number,json=x509svidSerialNumber,proto3" json:"x509svid_serial_number,omitempty"`
	// x509svid_expires_at field mask
	X509SvidExpiresAt bool `protobuf:"varint,4,opt,name=x509svid_expires_at,json=x509svidExpiresAt,proto3" json:"x509svid_expires_at,omitempty"`
	// selectors field mask
	Selectors bool `protobuf:"varint,5,opt,name=selectors,proto3" json:"selectors,omitempty"`
	// banned field mask
	Banned bool `protobuf:"varint,6,opt,name=banned,proto3" json:"banned,omitempty"`
}

func (x *AgentMask) Reset() {
	*x = AgentMask{}
	if protoimpl.UnsafeEnabled {
		mi := &file_spire_api_types_agent_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AgentMask) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AgentMask) ProtoMessage() {}

func (x *AgentMask) ProtoReflect() protoreflect.Message {
	mi := &file_spire_api_types_agent_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AgentMask.ProtoReflect.Descriptor instead.
func (*AgentMask) Descriptor() ([]byte, []int) {
	return file_spire_api_types_agent_proto_rawDescGZIP(), []int{1}
}

func (x *AgentMask) GetAttestationType() bool {
	if x != nil {
		return x.AttestationType
	}
	return false
}

func (x *AgentMask) GetX509SvidSerialNumber() bool {
	if x != nil {
		return x.X509SvidSerialNumber
	}
	return false
}

func (x *AgentMask) GetX509SvidExpiresAt() bool {
	if x != nil {
		return x.X509SvidExpiresAt
	}
	return false
}

func (x *AgentMask) GetSelectors() bool {
	if x != nil {
		return x.Selectors
	}
	return false
}

func (x *AgentMask) GetBanned() bool {
	if x != nil {
		return x.Banned
	}
	return false
}

var File_spire_api_types_agent_proto protoreflect.FileDescriptor

var file_spire_api_types_agent_proto_rawDesc = []byte{
	0x0a, 0x1b, 0x73, 0x70, 0x69, 0x72, 0x65, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x74, 0x79, 0x70, 0x65,
	0x73, 0x2f, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0f, 0x73,
	0x70, 0x69, 0x72, 0x65, 0x2e, 0x61, 0x70, 0x69, 0x2e, 0x74, 0x79, 0x70, 0x65, 0x73, 0x1a, 0x1e,
	0x73, 0x70, 0x69, 0x72, 0x65, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2f,
	0x73, 0x65, 0x6c, 0x65, 0x63, 0x74, 0x6f, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1e,
	0x73, 0x70, 0x69, 0x72, 0x65, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2f,
	0x73, 0x70, 0x69, 0x66, 0x66, 0x65, 0x69, 0x64, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x94,
	0x02, 0x0a, 0x05, 0x41, 0x67, 0x65, 0x6e, 0x74, 0x12, 0x29, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x19, 0x2e, 0x73, 0x70, 0x69, 0x72, 0x65, 0x2e, 0x61, 0x70, 0x69,
	0x2e, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x53, 0x50, 0x49, 0x46, 0x46, 0x45, 0x49, 0x44, 0x52,
	0x02, 0x69, 0x64, 0x12, 0x29, 0x0a, 0x10, 0x61, 0x74, 0x74, 0x65, 0x73, 0x74, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0f, 0x61,
	0x74, 0x74, 0x65, 0x73, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x54, 0x79, 0x70, 0x65, 0x12, 0x34,
	0x0a, 0x16, 0x78, 0x35, 0x30, 0x39, 0x73, 0x76, 0x69, 0x64, 0x5f, 0x73, 0x65, 0x72, 0x69, 0x61,
	0x6c, 0x5f, 0x6e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x14,
	0x78, 0x35, 0x30, 0x39, 0x73, 0x76, 0x69, 0x64, 0x53, 0x65, 0x72, 0x69, 0x61, 0x6c, 0x4e, 0x75,
	0x6d, 0x62, 0x65, 0x72, 0x12, 0x2e, 0x0a, 0x13, 0x78, 0x35, 0x30, 0x39, 0x73, 0x76, 0x69, 0x64,
	0x5f, 0x65, 0x78, 0x70, 0x69, 0x72, 0x65, 0x73, 0x5f, 0x61, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28,
	0x03, 0x52, 0x11, 0x78, 0x35, 0x30, 0x39, 0x73, 0x76, 0x69, 0x64, 0x45, 0x78, 0x70, 0x69, 0x72,
	0x65, 0x73, 0x41, 0x74, 0x12, 0x37, 0x0a, 0x09, 0x73, 0x65, 0x6c, 0x65, 0x63, 0x74, 0x6f, 0x72,
	0x73, 0x18, 0x05, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x19, 0x2e, 0x73, 0x70, 0x69, 0x72, 0x65, 0x2e,
	0x61, 0x70, 0x69, 0x2e, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x53, 0x65, 0x6c, 0x65, 0x63, 0x74,
	0x6f, 0x72, 0x52, 0x09, 0x73, 0x65, 0x6c, 0x65, 0x63, 0x74, 0x6f, 0x72, 0x73, 0x12, 0x16, 0x0a,
	0x06, 0x62, 0x61, 0x6e, 0x6e, 0x65, 0x64, 0x18, 0x06, 0x20, 0x01, 0x28, 0x08, 0x52, 0x06, 0x62,
	0x61, 0x6e, 0x6e, 0x65, 0x64, 0x22, 0xd2, 0x01, 0x0a, 0x09, 0x41, 0x67, 0x65, 0x6e, 0x74, 0x4d,
	0x61, 0x73, 0x6b, 0x12, 0x29, 0x0a, 0x10, 0x61, 0x74, 0x74, 0x65, 0x73, 0x74, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x08, 0x52, 0x0f, 0x61,
	0x74, 0x74, 0x65, 0x73, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x54, 0x79, 0x70, 0x65, 0x12, 0x34,
	0x0a, 0x16, 0x78, 0x35, 0x30, 0x39, 0x73, 0x76, 0x69, 0x64, 0x5f, 0x73, 0x65, 0x72, 0x69, 0x61,
	0x6c, 0x5f, 0x6e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x18, 0x03, 0x20, 0x01, 0x28, 0x08, 0x52, 0x14,
	0x78, 0x35, 0x30, 0x39, 0x73, 0x76, 0x69, 0x64, 0x53, 0x65, 0x72, 0x69, 0x61, 0x6c, 0x4e, 0x75,
	0x6d, 0x62, 0x65, 0x72, 0x12, 0x2e, 0x0a, 0x13, 0x78, 0x35, 0x30, 0x39, 0x73, 0x76, 0x69, 0x64,
	0x5f, 0x65, 0x78, 0x70, 0x69, 0x72, 0x65, 0x73, 0x5f, 0x61, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28,
	0x08, 0x52, 0x11, 0x78, 0x35, 0x30, 0x39, 0x73, 0x76, 0x69, 0x64, 0x45, 0x78, 0x70, 0x69, 0x72,
	0x65, 0x73, 0x41, 0x74, 0x12, 0x1c, 0x0a, 0x09, 0x73, 0x65, 0x6c, 0x65, 0x63, 0x74, 0x6f, 0x72,
	0x73, 0x18, 0x05, 0x20, 0x01, 0x28, 0x08, 0x52, 0x09, 0x73, 0x65, 0x6c, 0x65, 0x63, 0x74, 0x6f,
	0x72, 0x73, 0x12, 0x16, 0x0a, 0x06, 0x62, 0x61, 0x6e, 0x6e, 0x65, 0x64, 0x18, 0x06, 0x20, 0x01,
	0x28, 0x08, 0x52, 0x06, 0x62, 0x61, 0x6e, 0x6e, 0x65, 0x64, 0x42, 0x37, 0x5a, 0x35, 0x67, 0x69,
	0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x73, 0x70, 0x69, 0x66, 0x66, 0x65, 0x2f,
	0x73, 0x70, 0x69, 0x72, 0x65, 0x2d, 0x61, 0x70, 0x69, 0x2d, 0x73, 0x64, 0x6b, 0x2f, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x2f, 0x73, 0x70, 0x69, 0x72, 0x65, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x74, 0x79,
	0x70, 0x65, 0x73, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_spire_api_types_agent_proto_rawDescOnce sync.Once
	file_spire_api_types_agent_proto_rawDescData = file_spire_api_types_agent_proto_rawDesc
)

func file_spire_api_types_agent_proto_rawDescGZIP() []byte {
	file_spire_api_types_agent_proto_rawDescOnce.Do(func() {
		file_spire_api_types_agent_proto_rawDescData = protoimpl.X.CompressGZIP(file_spire_api_types_agent_proto_rawDescData)
	})
	return file_spire_api_types_agent_proto_rawDescData
}

var file_spire_api_types_agent_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_spire_api_types_agent_proto_goTypes = []interface{}{
	(*Agent)(nil),     // 0: spire.api.types.Agent
	(*AgentMask)(nil), // 1: spire.api.types.AgentMask
	(*SPIFFEID)(nil),  // 2: spire.api.types.SPIFFEID
	(*Selector)(nil),  // 3: spire.api.types.Selector
}
var file_spire_api_types_agent_proto_depIdxs = []int32{
	2, // 0: spire.api.types.Agent.id:type_name -> spire.api.types.SPIFFEID
	3, // 1: spire.api.types.Agent.selectors:type_name -> spire.api.types.Selector
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_spire_api_types_agent_proto_init() }
func file_spire_api_types_agent_proto_init() {
	if File_spire_api_types_agent_proto != nil {
		return
	}
	file_spire_api_types_selector_proto_init()
	file_spire_api_types_spiffeid_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_spire_api_types_agent_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Agent); i {
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
		file_spire_api_types_agent_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*AgentMask); i {
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
			RawDescriptor: file_spire_api_types_agent_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_spire_api_types_agent_proto_goTypes,
		DependencyIndexes: file_spire_api_types_agent_proto_depIdxs,
		MessageInfos:      file_spire_api_types_agent_proto_msgTypes,
	}.Build()
	File_spire_api_types_agent_proto = out.File
	file_spire_api_types_agent_proto_rawDesc = nil
	file_spire_api_types_agent_proto_goTypes = nil
	file_spire_api_types_agent_proto_depIdxs = nil
}
