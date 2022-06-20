// Code generated by protoc-gen-go-neofs. DO NOT EDIT.

package control

import "github.com/nspcc-dev/neofs-api-go/v2/util/proto"

// StableSize returns the size of x in protobuf format.
//
// Structures with the same field values have the same binary size.
func (x *HealthCheckRequest_Body) StableSize() (size int) {
	return size
}

// StableMarshal marshals x in protobuf binary format with stable field order.
//
// If buffer length is less than x.StableSize(), new buffer is allocated.
//
// Returns any error encountered which did not allow writing the data completely.
// Otherwise, returns the buffer in which the data is written.
//
// Structures with the same field values have the same binary format.
func (x *HealthCheckRequest_Body) StableMarshal(buf []byte) []byte {
	return buf
}

// StableSize returns the size of x in protobuf format.
//
// Structures with the same field values have the same binary size.
func (x *HealthCheckRequest) StableSize() (size int) {
	size += proto.NestedStructureSize(1, x.Body)
	size += proto.NestedStructureSize(2, x.Signature)
	return size
}

// StableMarshal marshals x in protobuf binary format with stable field order.
//
// If buffer length is less than x.StableSize(), new buffer is allocated.
//
// Returns any error encountered which did not allow writing the data completely.
// Otherwise, returns the buffer in which the data is written.
//
// Structures with the same field values have the same binary format.
func (x *HealthCheckRequest) StableMarshal(buf []byte) []byte {
	if x == nil {
		return []byte{}
	}
	if buf == nil {
		buf = make([]byte, x.StableSize())
	}
	var offset int
	offset += proto.NestedStructureMarshal(1, buf[offset:], x.Body)
	offset += proto.NestedStructureMarshal(2, buf[offset:], x.Signature)
	return buf
}

// ReadSignedData fills buf with signed data of x.
// If buffer length is less than x.SignedDataSize(), new buffer is allocated.
//
// Returns any error encountered which did not allow writing the data completely.
// Otherwise, returns the buffer in which the data is written.
//
// Structures with the same field values have the same signed data.
func (x *HealthCheckRequest) SignedDataSize() int {
	return x.GetBody().StableSize()
}

// SignedDataSize returns size of the request signed data in bytes.
//
// Structures with the same field values have the same signed data size.
func (x *HealthCheckRequest) ReadSignedData(buf []byte) ([]byte, error) {
	return x.GetBody().StableMarshal(buf), nil
}

func (x *HealthCheckRequest) SetSignature(sig *Signature) {
	x.Signature = sig
}

// StableSize returns the size of x in protobuf format.
//
// Structures with the same field values have the same binary size.
func (x *HealthCheckResponse_Body) StableSize() (size int) {
	size += proto.EnumSize(1, int32(x.HealthStatus))
	return size
}

// StableMarshal marshals x in protobuf binary format with stable field order.
//
// If buffer length is less than x.StableSize(), new buffer is allocated.
//
// Returns any error encountered which did not allow writing the data completely.
// Otherwise, returns the buffer in which the data is written.
//
// Structures with the same field values have the same binary format.
func (x *HealthCheckResponse_Body) StableMarshal(buf []byte) []byte {
	if x == nil {
		return []byte{}
	}
	if buf == nil {
		buf = make([]byte, x.StableSize())
	}
	var offset int
	offset += proto.EnumMarshal(1, buf[offset:], int32(x.HealthStatus))
	return buf
}

// StableSize returns the size of x in protobuf format.
//
// Structures with the same field values have the same binary size.
func (x *HealthCheckResponse) StableSize() (size int) {
	size += proto.NestedStructureSize(1, x.Body)
	size += proto.NestedStructureSize(2, x.Signature)
	return size
}

// StableMarshal marshals x in protobuf binary format with stable field order.
//
// If buffer length is less than x.StableSize(), new buffer is allocated.
//
// Returns any error encountered which did not allow writing the data completely.
// Otherwise, returns the buffer in which the data is written.
//
// Structures with the same field values have the same binary format.
func (x *HealthCheckResponse) StableMarshal(buf []byte) []byte {
	if x == nil {
		return []byte{}
	}
	if buf == nil {
		buf = make([]byte, x.StableSize())
	}
	var offset int
	offset += proto.NestedStructureMarshal(1, buf[offset:], x.Body)
	offset += proto.NestedStructureMarshal(2, buf[offset:], x.Signature)
	return buf
}

// ReadSignedData fills buf with signed data of x.
// If buffer length is less than x.SignedDataSize(), new buffer is allocated.
//
// Returns any error encountered which did not allow writing the data completely.
// Otherwise, returns the buffer in which the data is written.
//
// Structures with the same field values have the same signed data.
func (x *HealthCheckResponse) SignedDataSize() int {
	return x.GetBody().StableSize()
}

// SignedDataSize returns size of the request signed data in bytes.
//
// Structures with the same field values have the same signed data size.
func (x *HealthCheckResponse) ReadSignedData(buf []byte) ([]byte, error) {
	return x.GetBody().StableMarshal(buf), nil
}

func (x *HealthCheckResponse) SetSignature(sig *Signature) {
	x.Signature = sig
}
