package itrie

import "sync/atomic"

type Node[T any] struct {
	slot          [256]atomic.Pointer[Node[T]]
	childrenCount atomic.Int64
	data          *T
}

type ITrie[T any] struct {
	head *Node[T]
}

func New[T any]() *ITrie[T] {
	return &ITrie[T]{head: &Node[T]{}}
}

func (t *ITrie[T]) Insert(key uint64, value *T) {
	node := t.head
	for i := 0; i < 8; i++ {
		index := (key >> (56 - i*8)) & 0xff
		if node.slot[index].Load() == nil {
			node.slot[index].CompareAndSwap(nil, &Node[T]{})
		}
		nextNode := node.slot[index].Load()
		if nextNode.childrenCount.Load() == -1 {
			node.slot[index].CompareAndSwap(nextNode, &Node[T]{})
		}
		nextNode.childrenCount.Add(1)
		node = nextNode
	}
	node.data = value
}

func (t *ITrie[T]) Search(key uint64) *T {
	node := t.head
	for i := 0; i < 8; i++ {
		index := (key >> (56 - i*8)) & 0xff
		if node.slot[index].Load() == nil {
			return nil
		}
		node = node.slot[index].Load()
	}
	return node.data
}

func (t *ITrie[T]) Delete(key uint64) {
	node := t.head
	paths := make([]*Node[T], 0, 8)
	paths = append(paths, node)
	for i := 0; i < 8; i++ {
		index := (key >> (56 - i*8)) & 0xff
		if node.slot[index].Load() == nil {
			return
		}
		node = node.slot[index].Load()
		paths = append(paths, node)
	}
	node.data = nil
	for i := 7; i >= 0; i-- {
		if paths[i].childrenCount.Add(-1) == 0 && paths[i].childrenCount.CompareAndSwap(0, -1) {
		}
	}
	for i := 7; i >= 0; i-- {
		if paths[i].childrenCount.Load() == 0 && i > 0 {
			idx := (key >> (56 - (i-1)*8)) & 0xff
			if paths[i].childrenCount.CompareAndSwap(0, -1) {
				paths[i-1].slot[idx].CompareAndSwap(paths[i], nil)
			}
		}
	}
}
