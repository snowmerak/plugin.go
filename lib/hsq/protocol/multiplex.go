package protocol

import (
	"github.com/snowmerak/plugin.go/lib/hsq/protocol/itrie"
)

type MultiplexBuffer struct {
}

type Multiplexer struct {
	slot *itrie.ITrie[MultiplexBuffer]
}
