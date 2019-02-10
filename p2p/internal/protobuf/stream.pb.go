// Code generated by protoc-gen-go. DO NOT EDIT.
// source: stream.proto

package protobuf

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"
import _ "github.com/gogo/protobuf/gogoproto"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type ID struct {
	// public_key of the peer (we no longer use the public key as the peer ID, but use it to verify messages)
	PublicKey []byte `protobuf:"bytes,1,opt,name=public_key,json=publicKey,proto3" json:"public_key,omitempty"`
	// address is the network address of the peer
	Address string `protobuf:"bytes,2,opt,name=address,proto3" json:"address,omitempty"`
	// id is the computed hash of the public key
	Id                   []byte   `protobuf:"bytes,3,opt,name=id,proto3" json:"id,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ID) Reset()         { *m = ID{} }
func (m *ID) String() string { return proto.CompactTextString(m) }
func (*ID) ProtoMessage()    {}
func (*ID) Descriptor() ([]byte, []int) {
	return fileDescriptor_stream_4f891be46b934eb3, []int{0}
}
func (m *ID) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ID.Unmarshal(m, b)
}
func (m *ID) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ID.Marshal(b, m, deterministic)
}
func (dst *ID) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ID.Merge(dst, src)
}
func (m *ID) XXX_Size() int {
	return xxx_messageInfo_ID.Size(m)
}
func (m *ID) XXX_DiscardUnknown() {
	xxx_messageInfo_ID.DiscardUnknown(m)
}

var xxx_messageInfo_ID proto.InternalMessageInfo

func (m *ID) GetPublicKey() []byte {
	if m != nil {
		return m.PublicKey
	}
	return nil
}

func (m *ID) GetAddress() string {
	if m != nil {
		return m.Address
	}
	return ""
}

func (m *ID) GetId() []byte {
	if m != nil {
		return m.Id
	}
	return nil
}

type ChildBlock struct {
	SupervisorID           *ID        `protobuf:"bytes,1,opt,name=supervisorID,proto3" json:"supervisorID,omitempty"`
	Txs                    []byte     `protobuf:"bytes,2,opt,name=txs,proto3" json:"txs,omitempty"`
	NumTxs                 int64      `protobuf:"varint,3,opt,name=numTxs,proto3" json:"numTxs,omitempty"`
	Time                   *Timestamp `protobuf:"bytes,4,opt,name=time,proto3" json:"time,omitempty"`
	Signature              []byte     `protobuf:"bytes,5,opt,name=signature,proto3" json:"signature,omitempty"`
	ValidatorGroupHash     []byte     `protobuf:"bytes,6,opt,name=validatorGroupHash,proto3" json:"validatorGroupHash,omitempty"`
	NextValidatorGroupHash []byte     `protobuf:"bytes,7,opt,name=nextValidatorGroupHash,proto3" json:"nextValidatorGroupHash,omitempty"`
	XXX_NoUnkeyedLiteral   struct{}   `json:"-"`
	XXX_unrecognized       []byte     `json:"-"`
	XXX_sizecache          int32      `json:"-"`
}

func (m *ChildBlock) Reset()         { *m = ChildBlock{} }
func (m *ChildBlock) String() string { return proto.CompactTextString(m) }
func (*ChildBlock) ProtoMessage()    {}
func (*ChildBlock) Descriptor() ([]byte, []int) {
	return fileDescriptor_stream_4f891be46b934eb3, []int{1}
}
func (m *ChildBlock) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ChildBlock.Unmarshal(m, b)
}
func (m *ChildBlock) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ChildBlock.Marshal(b, m, deterministic)
}
func (dst *ChildBlock) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ChildBlock.Merge(dst, src)
}
func (m *ChildBlock) XXX_Size() int {
	return xxx_messageInfo_ChildBlock.Size(m)
}
func (m *ChildBlock) XXX_DiscardUnknown() {
	xxx_messageInfo_ChildBlock.DiscardUnknown(m)
}

var xxx_messageInfo_ChildBlock proto.InternalMessageInfo

func (m *ChildBlock) GetSupervisorID() *ID {
	if m != nil {
		return m.SupervisorID
	}
	return nil
}

func (m *ChildBlock) GetTxs() []byte {
	if m != nil {
		return m.Txs
	}
	return nil
}

func (m *ChildBlock) GetNumTxs() int64 {
	if m != nil {
		return m.NumTxs
	}
	return 0
}

func (m *ChildBlock) GetTime() *Timestamp {
	if m != nil {
		return m.Time
	}
	return nil
}

func (m *ChildBlock) GetSignature() []byte {
	if m != nil {
		return m.Signature
	}
	return nil
}

func (m *ChildBlock) GetValidatorGroupHash() []byte {
	if m != nil {
		return m.ValidatorGroupHash
	}
	return nil
}

func (m *ChildBlock) GetNextValidatorGroupHash() []byte {
	if m != nil {
		return m.NextValidatorGroupHash
	}
	return nil
}

type Message struct {
	Message []byte `protobuf:"bytes,1,opt,name=message,proto3" json:"message,omitempty"`
	// Sender's address and public key.
	Sender *ID `protobuf:"bytes,2,opt,name=sender,proto3" json:"sender,omitempty"`
	// Sender's signature of message.
	Signature []byte `protobuf:"bytes,3,opt,name=signature,proto3" json:"signature,omitempty"`
	// request_nonce is the request/response ID. Null if ID associated to a message is not a request/response.
	RequestNonce uint64 `protobuf:"varint,4,opt,name=request_nonce,json=requestNonce,proto3" json:"request_nonce,omitempty"`
	// message_nonce is the sequence ID.
	MessageNonce uint64 `protobuf:"varint,5,opt,name=message_nonce,json=messageNonce,proto3" json:"message_nonce,omitempty"`
	// reply_flag indicates this is a reply to a request
	ReplyFlag bool `protobuf:"varint,6,opt,name=reply_flag,json=replyFlag,proto3" json:"reply_flag,omitempty"`
	// opcode specifies the message type
	Opcode               uint32   `protobuf:"varint,7,opt,name=opcode,proto3" json:"opcode,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Message) Reset()         { *m = Message{} }
func (m *Message) String() string { return proto.CompactTextString(m) }
func (*Message) ProtoMessage()    {}
func (*Message) Descriptor() ([]byte, []int) {
	return fileDescriptor_stream_4f891be46b934eb3, []int{2}
}
func (m *Message) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Message.Unmarshal(m, b)
}
func (m *Message) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Message.Marshal(b, m, deterministic)
}
func (dst *Message) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Message.Merge(dst, src)
}
func (m *Message) XXX_Size() int {
	return xxx_messageInfo_Message.Size(m)
}
func (m *Message) XXX_DiscardUnknown() {
	xxx_messageInfo_Message.DiscardUnknown(m)
}

var xxx_messageInfo_Message proto.InternalMessageInfo

func (m *Message) GetMessage() []byte {
	if m != nil {
		return m.Message
	}
	return nil
}

func (m *Message) GetSender() *ID {
	if m != nil {
		return m.Sender
	}
	return nil
}

func (m *Message) GetSignature() []byte {
	if m != nil {
		return m.Signature
	}
	return nil
}

func (m *Message) GetRequestNonce() uint64 {
	if m != nil {
		return m.RequestNonce
	}
	return 0
}

func (m *Message) GetMessageNonce() uint64 {
	if m != nil {
		return m.MessageNonce
	}
	return 0
}

func (m *Message) GetReplyFlag() bool {
	if m != nil {
		return m.ReplyFlag
	}
	return false
}

func (m *Message) GetOpcode() uint32 {
	if m != nil {
		return m.Opcode
	}
	return 0
}

type Ping struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Ping) Reset()         { *m = Ping{} }
func (m *Ping) String() string { return proto.CompactTextString(m) }
func (*Ping) ProtoMessage()    {}
func (*Ping) Descriptor() ([]byte, []int) {
	return fileDescriptor_stream_4f891be46b934eb3, []int{3}
}
func (m *Ping) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Ping.Unmarshal(m, b)
}
func (m *Ping) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Ping.Marshal(b, m, deterministic)
}
func (dst *Ping) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Ping.Merge(dst, src)
}
func (m *Ping) XXX_Size() int {
	return xxx_messageInfo_Ping.Size(m)
}
func (m *Ping) XXX_DiscardUnknown() {
	xxx_messageInfo_Ping.DiscardUnknown(m)
}

var xxx_messageInfo_Ping proto.InternalMessageInfo

type Pong struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Pong) Reset()         { *m = Pong{} }
func (m *Pong) String() string { return proto.CompactTextString(m) }
func (*Pong) ProtoMessage()    {}
func (*Pong) Descriptor() ([]byte, []int) {
	return fileDescriptor_stream_4f891be46b934eb3, []int{4}
}
func (m *Pong) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Pong.Unmarshal(m, b)
}
func (m *Pong) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Pong.Marshal(b, m, deterministic)
}
func (dst *Pong) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Pong.Merge(dst, src)
}
func (m *Pong) XXX_Size() int {
	return xxx_messageInfo_Pong.Size(m)
}
func (m *Pong) XXX_DiscardUnknown() {
	xxx_messageInfo_Pong.DiscardUnknown(m)
}

var xxx_messageInfo_Pong proto.InternalMessageInfo

type LookupNodeRequest struct {
	Target               *ID      `protobuf:"bytes,1,opt,name=target,proto3" json:"target,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *LookupNodeRequest) Reset()         { *m = LookupNodeRequest{} }
func (m *LookupNodeRequest) String() string { return proto.CompactTextString(m) }
func (*LookupNodeRequest) ProtoMessage()    {}
func (*LookupNodeRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_stream_4f891be46b934eb3, []int{5}
}
func (m *LookupNodeRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_LookupNodeRequest.Unmarshal(m, b)
}
func (m *LookupNodeRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_LookupNodeRequest.Marshal(b, m, deterministic)
}
func (dst *LookupNodeRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_LookupNodeRequest.Merge(dst, src)
}
func (m *LookupNodeRequest) XXX_Size() int {
	return xxx_messageInfo_LookupNodeRequest.Size(m)
}
func (m *LookupNodeRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_LookupNodeRequest.DiscardUnknown(m)
}

var xxx_messageInfo_LookupNodeRequest proto.InternalMessageInfo

func (m *LookupNodeRequest) GetTarget() *ID {
	if m != nil {
		return m.Target
	}
	return nil
}

type LookupNodeResponse struct {
	Peers                []*ID    `protobuf:"bytes,1,rep,name=peers,proto3" json:"peers,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *LookupNodeResponse) Reset()         { *m = LookupNodeResponse{} }
func (m *LookupNodeResponse) String() string { return proto.CompactTextString(m) }
func (*LookupNodeResponse) ProtoMessage()    {}
func (*LookupNodeResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_stream_4f891be46b934eb3, []int{6}
}
func (m *LookupNodeResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_LookupNodeResponse.Unmarshal(m, b)
}
func (m *LookupNodeResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_LookupNodeResponse.Marshal(b, m, deterministic)
}
func (dst *LookupNodeResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_LookupNodeResponse.Merge(dst, src)
}
func (m *LookupNodeResponse) XXX_Size() int {
	return xxx_messageInfo_LookupNodeResponse.Size(m)
}
func (m *LookupNodeResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_LookupNodeResponse.DiscardUnknown(m)
}

var xxx_messageInfo_LookupNodeResponse proto.InternalMessageInfo

func (m *LookupNodeResponse) GetPeers() []*ID {
	if m != nil {
		return m.Peers
	}
	return nil
}

type Bytes struct {
	Data                 []byte   `protobuf:"bytes,1,opt,name=data,proto3" json:"data,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Bytes) Reset()         { *m = Bytes{} }
func (m *Bytes) String() string { return proto.CompactTextString(m) }
func (*Bytes) ProtoMessage()    {}
func (*Bytes) Descriptor() ([]byte, []int) {
	return fileDescriptor_stream_4f891be46b934eb3, []int{7}
}
func (m *Bytes) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Bytes.Unmarshal(m, b)
}
func (m *Bytes) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Bytes.Marshal(b, m, deterministic)
}
func (dst *Bytes) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Bytes.Merge(dst, src)
}
func (m *Bytes) XXX_Size() int {
	return xxx_messageInfo_Bytes.Size(m)
}
func (m *Bytes) XXX_DiscardUnknown() {
	xxx_messageInfo_Bytes.DiscardUnknown(m)
}

var xxx_messageInfo_Bytes proto.InternalMessageInfo

func (m *Bytes) GetData() []byte {
	if m != nil {
		return m.Data
	}
	return nil
}

// Timestamp wraps how amino encodes time.
// This is the protobuf well-known type protobuf/timestamp.proto
// See:
// https://github.com/google/protobuf/blob/d2980062c859649523d5fd51d6b55ab310e47482/src/google/protobuf/timestamp.proto#L123-L135
// NOTE/XXX: nanos do not get skipped if they are zero in amino.
type Timestamp struct {
	Seconds              int64    `protobuf:"varint,1,opt,name=seconds,proto3" json:"seconds,omitempty"`
	Nanos                int32    `protobuf:"varint,2,opt,name=nanos,proto3" json:"nanos,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Timestamp) Reset()         { *m = Timestamp{} }
func (m *Timestamp) String() string { return proto.CompactTextString(m) }
func (*Timestamp) ProtoMessage()    {}
func (*Timestamp) Descriptor() ([]byte, []int) {
	return fileDescriptor_stream_4f891be46b934eb3, []int{8}
}
func (m *Timestamp) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Timestamp.Unmarshal(m, b)
}
func (m *Timestamp) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Timestamp.Marshal(b, m, deterministic)
}
func (dst *Timestamp) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Timestamp.Merge(dst, src)
}
func (m *Timestamp) XXX_Size() int {
	return xxx_messageInfo_Timestamp.Size(m)
}
func (m *Timestamp) XXX_DiscardUnknown() {
	xxx_messageInfo_Timestamp.DiscardUnknown(m)
}

var xxx_messageInfo_Timestamp proto.InternalMessageInfo

func (m *Timestamp) GetSeconds() int64 {
	if m != nil {
		return m.Seconds
	}
	return 0
}

func (m *Timestamp) GetNanos() int32 {
	if m != nil {
		return m.Nanos
	}
	return 0
}

func init() {
	proto.RegisterType((*ID)(nil), "protobuf.ID")
	proto.RegisterType((*ChildBlock)(nil), "protobuf.ChildBlock")
	proto.RegisterType((*Message)(nil), "protobuf.Message")
	proto.RegisterType((*Ping)(nil), "protobuf.Ping")
	proto.RegisterType((*Pong)(nil), "protobuf.Pong")
	proto.RegisterType((*LookupNodeRequest)(nil), "protobuf.LookupNodeRequest")
	proto.RegisterType((*LookupNodeResponse)(nil), "protobuf.LookupNodeResponse")
	proto.RegisterType((*Bytes)(nil), "protobuf.Bytes")
	proto.RegisterType((*Timestamp)(nil), "protobuf.Timestamp")
}

func init() { proto.RegisterFile("stream.proto", fileDescriptor_stream_4f891be46b934eb3) }

var fileDescriptor_stream_4f891be46b934eb3 = []byte{
	// 561 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x74, 0x52, 0x4d, 0x6f, 0xd4, 0x30,
	0x10, 0xc5, 0xbb, 0x9b, 0x6d, 0x77, 0x48, 0x11, 0x18, 0x54, 0x45, 0x40, 0xa3, 0x2a, 0x20, 0xd1,
	0x0b, 0x29, 0x02, 0x09, 0x81, 0xb8, 0x2d, 0x55, 0xa1, 0x82, 0x56, 0x95, 0x55, 0x71, 0xad, 0xbc,
	0xc9, 0x34, 0xb5, 0x9a, 0xd8, 0xc1, 0x76, 0xaa, 0xee, 0x8d, 0x7f, 0xc1, 0x5f, 0xe0, 0xa7, 0x70,
	0xe4, 0xc8, 0xb1, 0xdd, 0x13, 0x47, 0x7e, 0x02, 0x8a, 0x9d, 0x2d, 0xf4, 0x83, 0x93, 0xe7, 0xbd,
	0x79, 0xcf, 0xf6, 0x3c, 0x1b, 0x42, 0x63, 0x35, 0xf2, 0x2a, 0xad, 0xb5, 0xb2, 0x8a, 0x2e, 0xba,
	0x65, 0xd2, 0x1c, 0xdc, 0x7f, 0x5a, 0x08, 0x7b, 0xd8, 0x4c, 0xd2, 0x4c, 0x55, 0xeb, 0x85, 0x2a,
	0xd4, 0xfa, 0xbc, 0xe3, 0x90, 0x03, 0xae, 0xf2, 0xc6, 0x64, 0x1b, 0x7a, 0x5b, 0x1b, 0x74, 0x05,
	0xa0, 0x6e, 0x26, 0xa5, 0xc8, 0xf6, 0x8f, 0x70, 0x1a, 0x91, 0x55, 0xb2, 0x16, 0xb2, 0x91, 0x67,
	0x3e, 0xe0, 0x94, 0x46, 0xb0, 0xc0, 0xf3, 0x5c, 0xa3, 0x31, 0x51, 0x6f, 0x95, 0xac, 0x8d, 0xd8,
	0x1c, 0xd2, 0x5b, 0xd0, 0x13, 0x79, 0xd4, 0x77, 0x86, 0x9e, 0xc8, 0x93, 0xaf, 0x3d, 0x80, 0xb7,
	0x87, 0xa2, 0xcc, 0xc7, 0xa5, 0xca, 0x8e, 0xe8, 0x33, 0x08, 0x4d, 0x53, 0xa3, 0x3e, 0x16, 0x46,
	0xe9, 0xad, 0x0d, 0xb7, 0xf3, 0xcd, 0xe7, 0x61, 0x3a, 0xbf, 0x53, 0xba, 0xb5, 0xc1, 0x2e, 0x28,
	0xe8, 0x6d, 0xe8, 0xdb, 0x13, 0x7f, 0x4c, 0xc8, 0xda, 0x92, 0x2e, 0xc3, 0x50, 0x36, 0xd5, 0xde,
	0x89, 0x71, 0xc7, 0xf4, 0x59, 0x87, 0xe8, 0x13, 0x18, 0x58, 0x51, 0x61, 0x34, 0x70, 0x7b, 0xde,
	0xfd, 0xbb, 0xe7, 0x9e, 0xa8, 0xd0, 0x58, 0x5e, 0xd5, 0xcc, 0x09, 0xe8, 0x43, 0x18, 0x19, 0x51,
	0x48, 0x6e, 0x1b, 0x8d, 0x51, 0xe0, 0x67, 0x3b, 0x27, 0x68, 0x0a, 0xf4, 0x98, 0x97, 0x22, 0xe7,
	0x56, 0xe9, 0x77, 0x5a, 0x35, 0xf5, 0x7b, 0x6e, 0x0e, 0xa3, 0xa1, 0x93, 0x5d, 0xd3, 0xa1, 0x2f,
	0x61, 0x59, 0xe2, 0x89, 0xfd, 0x74, 0xd5, 0xb3, 0xe0, 0x3c, 0xff, 0xe9, 0x26, 0xbf, 0x08, 0x2c,
	0x6c, 0xa3, 0x31, 0xbc, 0xc0, 0x36, 0xcf, 0xca, 0x97, 0x5d, 0xd6, 0x73, 0x48, 0x1f, 0xc3, 0xd0,
	0xa0, 0xcc, 0x51, 0xbb, 0x04, 0x2e, 0x47, 0xd5, 0xf5, 0x2e, 0x4e, 0xd4, 0xbf, 0x3c, 0xd1, 0x23,
	0x58, 0xd2, 0xf8, 0xb9, 0x41, 0x63, 0xf7, 0xa5, 0x92, 0x99, 0x4f, 0x68, 0xc0, 0xc2, 0x8e, 0xdc,
	0x69, 0xb9, 0x56, 0xd4, 0x9d, 0xd9, 0x89, 0x02, 0x2f, 0xea, 0x48, 0x2f, 0x5a, 0x01, 0xd0, 0x58,
	0x97, 0xd3, 0xfd, 0x83, 0x92, 0x17, 0x2e, 0x93, 0x45, 0x36, 0x72, 0xcc, 0x66, 0xc9, 0x8b, 0xf6,
	0x65, 0x54, 0x9d, 0xa9, 0x1c, 0xdd, 0xe8, 0x4b, 0xac, 0x43, 0xc9, 0x10, 0x06, 0xbb, 0x42, 0x16,
	0x6e, 0x55, 0xb2, 0x48, 0x5e, 0xc3, 0x9d, 0x8f, 0x4a, 0x1d, 0x35, 0xf5, 0x8e, 0xca, 0x91, 0xf9,
	0x5b, 0xb4, 0x93, 0x5a, 0xae, 0x0b, 0xb4, 0xd7, 0x7e, 0x8a, 0xae, 0x97, 0xbc, 0x02, 0xfa, 0xaf,
	0xd5, 0xd4, 0x4a, 0x1a, 0xa4, 0x09, 0x04, 0x35, 0xa2, 0x36, 0x11, 0x59, 0xed, 0x5f, 0xb1, 0xfa,
	0x56, 0xf2, 0x00, 0x82, 0xf1, 0xd4, 0xa2, 0xa1, 0x14, 0x06, 0x39, 0xb7, 0xbc, 0x4b, 0xda, 0xd5,
	0xc9, 0x1b, 0x18, 0x9d, 0xff, 0x92, 0xf6, 0x35, 0x0c, 0x66, 0x4a, 0xe6, 0xc6, 0x69, 0xfa, 0x6c,
	0x0e, 0xe9, 0x3d, 0x08, 0x24, 0x97, 0xca, 0x7f, 0xc7, 0x80, 0x79, 0x30, 0xde, 0xfc, 0x79, 0x16,
	0xdf, 0x38, 0x3d, 0x8b, 0xc9, 0xef, 0xb3, 0x98, 0x7c, 0x99, 0xc5, 0xe4, 0xdb, 0x2c, 0x26, 0xdf,
	0x67, 0x31, 0xf9, 0x31, 0x8b, 0xc9, 0xe9, 0x2c, 0x26, 0xb0, 0xac, 0x74, 0x91, 0xd6, 0xa8, 0x4b,
	0x21, 0x53, 0xa9, 0x84, 0x41, 0x7f, 0xc1, 0x31, 0xec, 0xb4, 0x60, 0xb7, 0xad, 0x77, 0xc9, 0x64,
	0xe8, 0xc8, 0x17, 0x7f, 0x02, 0x00, 0x00, 0xff, 0xff, 0xfc, 0xa8, 0x7b, 0x4a, 0xca, 0x03, 0x00,
	0x00,
}
