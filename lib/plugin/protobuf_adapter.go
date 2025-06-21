package plugin

import (
	"google.golang.org/protobuf/proto"
)

// NewProtobufLoaderAdapter creates a LoaderAdapter specialized for Protocol Buffers serialization.
// Req and Resp types must implement proto.Message (e.g., *pb.MyRequest, *pb.MyResponse).
// newRespInstance is a factory function that returns a new, non-nil instance of the Resp type.
// Example: newRespInstance := func() *pb.MyResponse { return new(pb.MyResponse) }
func NewProtobufLoaderAdapter[Req proto.Message, Resp proto.Message](
	loader *Loader,
	newRespInstance func() Resp, // Factory function for the response type
) *LoaderAdapter[Req, Resp] {
	if newRespInstance == nil {
		// This would lead to panic/error later. Better to catch it early,
		// though the caller is responsible for providing a valid factory.
		// Consider returning an error or panicking if loader is also nil.
		// For now, assume valid inputs or let later stages fail.
	}

	serializer := Serializer[Req, Resp]{
		MarshalRequest: func(req Req) ([]byte, error) {
			return proto.Marshal(req)
		},
		UnmarshalResponse: func(data []byte) (Resp, error) {
			instance := newRespInstance()
			// proto.Reset(instance) // Consider if instances from factory could be dirty; new ones usually aren't.
			err := proto.Unmarshal(data, instance) // instance must be a non-nil pointer to a proto struct
			if err != nil {
				var zero Resp
				return zero, err
			}
			return instance, nil
		},
		// UnmarshalError: func(data []byte) (string, error) {
		// 	// Default behavior from loader.Call is string(errorPayload)
		// 	// If errors are specific protobuf messages, unmarshal here.
		// 	return string(data), nil
		// },
	}
	return NewLoaderAdapter(loader, serializer)
}
