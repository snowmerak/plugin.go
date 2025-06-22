package main

import (
	"fmt"
	"os"
	"unsafe"

	"github.com/snowmerak/plugin.go/lib/hsq/mmap"
	"github.com/snowmerak/plugin.go/lib/hsq/offheap/bufring"
	"github.com/snowmerak/plugin.go/lib/hsq/shm"
)

func main() {
	fmt.Println("=== HSQ Basic Test ===")

	// Test shared memory creation
	name := "/hsq_basic_test"
	size := 8192 // 8KB for simple test

	fmt.Printf("Creating shared memory: %s (size: %d)\n", name, size)

	sm, err := shm.OpenSharedMemory(name, size, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		fmt.Printf("Failed to create shared memory: %v\n", err)
		return
	}
	defer func() {
		sm.Close()
		sm.Delete()
		fmt.Println("Cleaned up shared memory")
	}()

	fmt.Println("Shared memory created successfully")

	// Test memory mapping
	fmt.Println("Mapping memory...")
	memory, err := mmap.Map(sm.FD(), 0, size, mmap.PROT_READ|mmap.PROT_WRITE, mmap.MAP_SHARED)
	if err != nil {
		fmt.Printf("Failed to map memory: %v\n", err)
		return
	}
	defer func() {
		mmap.UnMap(memory)
		fmt.Println("Unmapped memory")
	}()

	fmt.Println("Memory mapped successfully")

	// Test buffer ring creation
	fmt.Println("Creating buffer ring...")
	ringSize := 8
	maxMsgSize := 256

	basePtr := uintptr(unsafe.Pointer(&memory[0]))
	ring := bufring.NewBufferRing(basePtr, ringSize, maxMsgSize, false)

	fmt.Printf("Buffer ring created: ring_size=%d, max_msg_size=%d\n", ring.Size(), ring.MaxBufferSize())

	// Test send/receive
	fmt.Println("Testing send/receive...")
	testMsg := "Hello, HSQ!"

	ring.Send(func(b []byte) []byte {
		copy(b, []byte(testMsg))
		return b[:len(testMsg)]
	})

	ring.Receive(func(data []byte) {
		received := string(data)
		fmt.Printf("Sent: %s, Received: %s\n", testMsg, received)
		if received == testMsg {
			fmt.Println("✓ HSQ basic test passed!")
		} else {
			fmt.Printf("✗ HSQ basic test failed: expected %s, got %s\n", testMsg, received)
		}
	})
}
