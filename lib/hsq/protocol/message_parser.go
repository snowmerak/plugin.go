package protocol

import (
	"bytes"
	"context"
	"errors"
	"io"
)

type CommonHeader struct {
	MessageType        int8   `json:"message_type"`
	Sender             uint32 `json:"sender"`
	MessageID          uint32 `json:"message_id"`
	MessageMaxSequence uint16 `json:"message_max_sequence"`
	MessageSequence    uint16 `json:"message_sequence"`
}

func (h *CommonHeader) Key() (uint32, uint32) {
	return h.Sender, h.MessageID
}

func (h *CommonHeader) Parse(ctx context.Context, chunkSize int, message Message, callbackPerSeq func(context.Context, []byte) error) error {
	headerLength := 1 + 4 + 4 + 8 + 2 + 2
	bw := bytes.NewBuffer(make([]byte, headerLength))
	if err := SerializeInt8(bw, h.MessageType); err != nil {
		return err
	}
	if err := SerializeUint32(bw, h.Sender); err != nil {
		return err
	}
	if err := SerializeUint32(bw, h.MessageID); err != nil {
		return err
	}

	bodyBuffer := bytes.NewBuffer(nil)
	if err := message.Serialize(bodyBuffer); err != nil {
		return err
	}

	bodySize := bodyBuffer.Len()
	messageSequenceValue := bodySize / chunkSize
	if bodySize%chunkSize != 0 {
		messageSequenceValue++
	}

	messageSequence := uint16(messageSequenceValue)
	messageMaxSequence := uint16(messageSequenceValue)

	if err := SerializeUint16(bw, messageMaxSequence); err != nil {
		return err
	}

	fixedHeaderLength := bw.Len()

	for bodyBuffer.Len() > 0 {
		chunk := make([]byte, chunkSize)
		if err := SerializeUint16(bw, messageSequence); err != nil {
			return err
		}
		messageSequence++

		copy(chunk, bw.Bytes())
		headerLength = bw.Len()
		bw.Truncate(fixedHeaderLength)

		_, err := bodyBuffer.Read(chunk[headerLength:])
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}

		if err := callbackPerSeq(ctx, chunk); err != nil {
			return err
		}
	}

	return nil
}

func SerializeCommonHeader(r io.Reader, header *CommonHeader) error {
	mt, err := DeserializeInt8(r)
	if err != nil {
		return err
	}

	header.MessageType = mt

	sender, err := DeserializeUint32(r)
	if err != nil {
		return err
	}

	header.Sender = sender

	messageID, err := DeserializeUint32(r)
	if err != nil {
		return err
	}

	header.MessageID = messageID

	messageMaxSequence, err := DeserializeUint16(r)
	if err != nil {
		return err
	}

	header.MessageMaxSequence = messageMaxSequence

	messageSequence, err := DeserializeUint16(r)
	if err != nil {
		return err
	}

	header.MessageSequence = messageSequence

	return nil
}

type Message interface {
	Serialize(w io.Writer) error
	Deserialize(w io.Writer, r io.Reader) error
}

type Hello struct {
	ProtocolVersionMajor uint8  `json:"protocol_version"`
	ProtocolVersionMinor uint8  `json:"protocol_version_minor"`
	ProtocolVersionPatch uint8  `json:"protocol_version_patch"`
	PluginName           string `json:"plugin_name"`
	PluginVersion        int64  `json:"plugin_version"`
	SharedMemoryName     string `json:"shared_memory_name"`
}

func (h *Hello) Serialize(w io.Writer) error {
	if err := SerializeUint8(w, h.ProtocolVersionMajor); err != nil {
		return err
	}
	if err := SerializeUint8(w, h.ProtocolVersionMinor); err != nil {
		return err
	}
	if err := SerializeUint8(w, h.ProtocolVersionPatch); err != nil {
		return err
	}
	if err := SerializeString(w, h.PluginName); err != nil {
		return err
	}
	if err := SerializeInt64(w, h.PluginVersion); err != nil {
		return err
	}
	if err := SerializeString(w, h.SharedMemoryName); err != nil {
		return err
	}

	return nil
}

func (h *Hello) Deserialize(w io.Writer, r io.Reader) error {
	protocolVersionMajor, err := DeserializeUint8(r)
	if err != nil {
		return err
	}

	h.ProtocolVersionMajor = protocolVersionMajor

	protocolVersionMinor, err := DeserializeUint8(r)
	if err != nil {
		return err
	}

	h.ProtocolVersionMinor = protocolVersionMinor

	protocolVersionPatch, err := DeserializeUint8(r)
	if err != nil {
		return err
	}

	h.ProtocolVersionPatch = protocolVersionPatch

	pluginName, err := DeserializeString(r)
	if err != nil {
		return err
	}

	h.PluginName = pluginName

	pluginVersion, err := DeserializeInt64(r)
	if err != nil {
		return err
	}

	h.PluginVersion = pluginVersion

	sharedMemoryName, err := DeserializeString(r)
	if err != nil {
		return err
	}

	h.SharedMemoryName = sharedMemoryName

	return nil
}

type Bye struct {
	Reason string `json:"reason"`
}

func (b *Bye) Serialize(w io.Writer) error {
	if err := SerializeString(w, b.Reason); err != nil {
		return err
	}

	return nil
}

func (b *Bye) Deserialize(w io.Writer, r io.Reader) error {
	reason, err := DeserializeString(r)
	if err != nil {
		return err
	}

	b.Reason = reason

	return nil
}

type Request struct {
	RequestID   uint64 `json:"request_id"`
	Method      string `json:"method"`
	RequestBody []byte `json:"request_body"`
}

func (r *Request) Serialize(w io.Writer) error {
	if err := SerializeUint64(w, r.RequestID); err != nil {
		return err
	}
	if err := SerializeString(w, r.Method); err != nil {
		return err
	}
	if err := SerializeBytes(w, r.RequestBody); err != nil {
		return err
	}

	return nil
}

func (r *Request) Deserialize(w io.Writer, reader io.Reader) error {
	requestID, err := DeserializeUint64(reader)
	if err != nil {
		return err
	}

	r.RequestID = requestID

	method, err := DeserializeString(reader)
	if err != nil {
		return err
	}

	r.Method = method

	requestBody, err := DeserializeBytes(reader)
	if err != nil {
		return err
	}

	r.RequestBody = requestBody

	return nil
}

type Response struct {
	RequestID    uint64 `json:"request_id"`
	ResponseBody []byte `json:"response_body"`
}

func (r *Response) Serialize(w io.Writer) error {
	if err := SerializeUint64(w, r.RequestID); err != nil {
		return err
	}
	if err := SerializeBytes(w, r.ResponseBody); err != nil {
		return err
	}

	return nil
}

func (r *Response) Deserialize(w io.Writer, reader io.Reader) error {
	requestID, err := DeserializeUint64(reader)
	if err != nil {
		return err
	}

	r.RequestID = requestID

	responseBody, err := DeserializeBytes(reader)
	if err != nil {
		return err
	}

	r.ResponseBody = responseBody

	return nil
}

type Ping struct {
	PingID    uint64 `json:"ping_id"`
	Timestamp uint64 `json:"timestamp"`
}

func (p *Ping) Serialize(w io.Writer) error {
	if err := SerializeUint64(w, p.PingID); err != nil {
		return err
	}
	if err := SerializeUint64(w, p.Timestamp); err != nil {
		return err
	}

	return nil
}

func (p *Ping) Deserialize(w io.Writer, reader io.Reader) error {
	pingID, err := DeserializeUint64(reader)
	if err != nil {
		return err
	}

	p.PingID = pingID

	timestamp, err := DeserializeUint64(reader)
	if err != nil {
		return err
	}

	p.Timestamp = timestamp

	return nil
}

type Pong struct {
	PingID    uint64 `json:"ping_id"`
	Timestamp uint64 `json:"timestamp"`
}

func (p *Pong) Serialize(w io.Writer) error {
	if err := SerializeUint64(w, p.PingID); err != nil {
		return err
	}
	if err := SerializeUint64(w, p.Timestamp); err != nil {
		return err
	}

	return nil
}

func (p *Pong) Deserialize(w io.Writer, reader io.Reader) error {
	pingID, err := DeserializeUint64(reader)
	if err != nil {
		return err
	}

	p.PingID = pingID

	timestamp, err := DeserializeUint64(reader)
	if err != nil {
		return err
	}

	p.Timestamp = timestamp

	return nil
}
