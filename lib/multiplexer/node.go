package multiplexer

import (
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

type Node struct {
	reader *io.PipeReader
	writer *io.PipeWriter

	writerLock *sync.Mutex

	readBuffer map[uint64]*Message

	sequence atomic.Uint64
}

func NewNode(reader *io.PipeReader, writer *io.PipeWriter) *Node {
	return &Node{
		reader:     reader,
		writer:     writer,
		writerLock: &sync.Mutex{},
		readBuffer: make(map[uint64]*Message),
	}
}

const (
	// 1 Byte for the message type, 4 Bytes for the frame sequence, and 4 Bytes for the data length
	MessageHeaderSize         = 9           // Size of the message ID in bytes
	MessageHeaderTypeStart    = uint8(0x01) // Start of a message
	MessageHeaderTypeEnd      = uint8(0x02) // End of a message
	MessageHeaderTypeData     = uint8(0x03) // Data part of a message
	MessageHeaderTypeError    = uint8(0x04) // Error message
	MessageHeaderTypeComplete = uint8(0x05) // Complete message (all parts received and processed)
	MessageHeaderTypeAbort    = uint8(0x06) // Abort message
)

const (
	MessageChunkSize = 1024 // Size of each chunk for reading data
)

type Message struct {
	ID   uint64
	Data []byte
	Type uint8
}

func (n *Node) ReadMessage(ctx context.Context) (chan *Message, error) {
	const defaultMaxBufferLength = 4096
	ch := make(chan *Message, defaultMaxBufferLength)
	go func() {
		defer close(ch)
		const (
			defaultMaxByteBufferSize = 1024 * 1024 // 1 MB
		)
		buffer := make([]byte, defaultMaxByteBufferSize)
		done := ctx.Done()
	loop:
		for {
			select {
			case <-done:
				ch <- &Message{Type: MessageHeaderTypeError, Data: []byte("context done")}
				return
			default:
				temp := buffer[:MessageHeaderSize]
				c, err := io.ReadFull(n.reader, temp)
				if err != nil {
					if err == io.EOF {
						return // End of stream
					}
					ch <- &Message{Type: MessageHeaderTypeError, Data: []byte(err.Error())} // Send error as message
					continue loop
				}

				if c < MessageHeaderSize {
					ch <- &Message{Type: MessageHeaderTypeError, Data: []byte("incomplete header")}
					continue loop
				}

				msgType := temp[0]
				frameID := uint64(temp[1])<<24 | uint64(temp[2])<<16 | uint64(temp[3])<<8 | uint64(temp[4])
				dataLength := uint64(temp[5])<<24 | uint64(temp[6])<<16 | uint64(temp[7])<<8 | uint64(temp[8])

				if len(buffer) < int(dataLength) {
					buffer = make([]byte, dataLength)
				}

				switch msgType {
				case MessageHeaderTypeStart:
					m := &Message{
						ID:   frameID,
						Type: MessageHeaderTypeStart,
						Data: make([]byte, 0, dataLength),
					}

					n.readBuffer[frameID] = m
				case MessageHeaderTypeData:
					c, err = io.ReadFull(n.reader, buffer[:dataLength])
					if err != nil {
						if err == io.EOF {
							return // End of stream
						}
						ch <- &Message{Type: MessageHeaderTypeError, Data: []byte(err.Error())} // Send error as message
						continue loop
					}

					if c < int(dataLength) {
						ch <- &Message{Type: MessageHeaderTypeError, Data: []byte("incomplete data")}
						continue loop
					}

					m, ok := n.readBuffer[frameID]
					if !ok {
						ch <- &Message{Type: MessageHeaderTypeError, Data: []byte("unknown frame ID")}
						continue loop
					}

					m.Data = append(m.Data, buffer[:dataLength]...)
				case MessageHeaderTypeEnd:
					m, ok := n.readBuffer[frameID]
					if !ok {
						ch <- &Message{Type: MessageHeaderTypeError, Data: []byte("unknown frame ID")}
						continue loop
					}
					m.Type = MessageHeaderTypeComplete
					ch <- m
					delete(n.readBuffer, frameID)
				case MessageHeaderTypeAbort:
					m, ok := n.readBuffer[frameID]
					if !ok {
						ch <- &Message{Type: MessageHeaderTypeError, Data: []byte("unknown frame ID")}
						continue loop
					}
					m.Type = MessageHeaderTypeAbort
					ch <- m
					delete(n.readBuffer, frameID)
				default:
					ch <- &Message{Type: MessageHeaderTypeError, Data: []byte(fmt.Sprintf("unknown message type: %d", msgType))}
					continue loop
				}
			}
		}
	}()

	return ch, nil
}

func (n *Node) write(mesgType uint8, frameID uint64, data []byte) error {
	n.writerLock.Lock()
	defer n.writerLock.Unlock()
	if n.writer == nil {
		return fmt.Errorf("writer is nil")
	}

	if len(data) > 0xFFFFFFFF {
		return fmt.Errorf("data length exceeds maximum size")
	}

	header := make([]byte, MessageHeaderSize)
	header[0] = mesgType
	header[1] = byte(frameID >> 24)
	header[2] = byte(frameID >> 16)
	header[3] = byte(frameID >> 8)
	header[4] = byte(frameID)
	header[5] = byte(len(data) >> 24)
	header[6] = byte(len(data) >> 16)
	header[7] = byte(len(data) >> 8)
	header[8] = byte(len(data))
	if _, err := n.writer.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	switch mesgType {
	case MessageHeaderTypeStart:
		// No additional data to write for start message
	case MessageHeaderTypeData:
		if len(data) > 0 {
			if _, err := n.writer.Write(data); err != nil {
				return fmt.Errorf("failed to write data: %w", err)
			}
		}
	case MessageHeaderTypeEnd:
	// No additional data to write for end message
	case MessageHeaderTypeAbort:
		// No additional data to write for abort message
	}

	return nil
}

func (n *Node) WriteResponseMessage(ctx context.Context, seq uint64, data []byte) error {
	done := ctx.Done()

	if err := n.write(MessageHeaderTypeStart, seq, data); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	select {
	case <-done:
		if err := n.write(MessageHeaderTypeAbort, seq, nil); err != nil {
			return fmt.Errorf("failed to write abort message: %w", err)
		}
		return ctx.Err()
	default:
	}

	for len(data) > 0 {
		chunkSize := min(len(data), MessageChunkSize)

		if err := n.write(MessageHeaderTypeData, seq, data[:chunkSize]); err != nil {
			return fmt.Errorf("failed to write data chunk: %w", err)
		}

		select {
		case <-done:
			if err := n.write(MessageHeaderTypeAbort, seq, nil); err != nil {
				return fmt.Errorf("failed to write abort message: %w", err)
			}
			return ctx.Err()
		default:
		}

		data = data[chunkSize:]
	}

	select {
	case <-done:
		if err := n.write(MessageHeaderTypeAbort, seq, nil); err != nil {
			return fmt.Errorf("failed to write abort message: %w", err)
		}
		return ctx.Err()
	default:
	}

	if err := n.write(MessageHeaderTypeEnd, seq, nil); err != nil {
		return fmt.Errorf("failed to write end message: %w", err)
	}

	return nil
}

func (n *Node) WriteRequestMessage(ctx context.Context, data []byte) error {
	if err := n.write(MessageHeaderTypeStart, n.sequence.Add(1), data); err != nil {
		return fmt.Errorf("failed to write request message: %w", err)
	}

	return nil
}
