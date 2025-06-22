// Package plugin provides backward compatibility wrappers for the Module functionality.
//
// This file contains wrapper implementations that maintain compatibility
// with older Node interfaces while using the new multiplexer system.
package plugin

import (
	"context"
	"sync"

	"github.com/snowmerak/plugin.go/lib/multiplexer"
)

// oldNodeWrapper wraps the original Node to provide our interface.
// This is used for compatibility with the old API.
type oldNodeWrapper struct {
	node *multiplexer.Node
}

// WriteMessage wraps the Node's WriteMessage method.
func (n *oldNodeWrapper) WriteMessage(ctx context.Context, data []byte) error {
	return n.node.WriteMessage(ctx, data)
}

// WriteMessageWithSequence wraps the Node's WriteMessageWithSequence method.
func (n *oldNodeWrapper) WriteMessageWithSequence(ctx context.Context, seq uint32, data []byte) error {
	return n.node.WriteMessageWithSequence(ctx, seq, data)
}

// ReadMessage wraps the Node's ReadMessage and converts to old format.
func (n *oldNodeWrapper) ReadMessage(ctx context.Context) (chan *OldMessage, error) {
	newCh, err := n.node.ReadMessage(ctx)
	if err != nil {
		return nil, err
	}

	oldCh := make(chan *OldMessage, 100)
	go func() {
		defer close(oldCh)
		for newMsg := range newCh {
			oldMsg := &OldMessage{
				ID:   newMsg.ID,
				Data: newMsg.Data,
				Type: newMsg.Type,
			}
			oldCh <- oldMsg
		}
	}()

	return oldCh, nil
}

// nodeWrapper wraps the new multiplexer API to provide the old Node interface.
type nodeWrapper struct {
	multiplexer multiplexer.Multiplexer
	sequence    uint32
	mu          sync.Mutex
}

// OldMessage represents a message in the old format for backward compatibility.
type OldMessage struct {
	ID   uint32
	Data []byte
	Type uint8
}

// WriteMessage wraps the new API for writing messages.
func (n *nodeWrapper) WriteMessage(ctx context.Context, data []byte) error {
	return n.multiplexer.WriteMessage(ctx, data)
}

// WriteMessageWithSequence wraps the new API for writing messages with sequence.
func (n *nodeWrapper) WriteMessageWithSequence(ctx context.Context, seq uint32, data []byte) error {
	return n.multiplexer.WriteMessageWithSequence(ctx, seq, data)
}

// ReadMessage wraps the new API and converts to old format.
func (n *nodeWrapper) ReadMessage(ctx context.Context) (chan *OldMessage, error) {
	newCh, err := n.multiplexer.ReadMessage(ctx)
	if err != nil {
		return nil, err
	}

	oldCh := make(chan *OldMessage, 100)
	go func() {
		defer close(oldCh)
		for newMsg := range newCh {
			oldMsg := &OldMessage{
				ID:   newMsg.Sequence,
				Data: newMsg.Data,
				Type: 0x05, // MessageHeaderTypeComplete
			}
			oldCh <- oldMsg
		}
	}()

	return oldCh, nil
}
