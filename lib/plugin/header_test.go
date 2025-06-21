package plugin

import (
	"bytes"
	"testing"
)

func TestHeader_MarshalUnmarshal(t *testing.T) {
	testCases := []struct {
		name   string
		header Header
	}{
		{
			name: "Simple header",
			header: Header{
				Name:    "test",
				IsError: false,
				Payload: []byte("hello world"),
			},
		},
		{
			name: "Error header",
			header: Header{
				Name:    "error_service",
				IsError: true,
				Payload: []byte("error message"),
			},
		},
		{
			name: "Empty payload",
			header: Header{
				Name:    "empty",
				IsError: false,
				Payload: []byte{},
			},
		},
		{
			name: "Long name",
			header: Header{
				Name:    "very_long_service_name_for_testing_purposes",
				IsError: false,
				Payload: []byte("test data"),
			},
		},
		{
			name: "Large payload",
			header: Header{
				Name:    "large",
				IsError: false,
				Payload: make([]byte, 10000), // 10KB payload
			},
		},
		{
			name: "Unicode name",
			header: Header{
				Name:    "test_service",
				IsError: false,
				Payload: []byte("unicode test"),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Marshal
			data, err := tc.header.MarshalBinary()
			if err != nil {
				t.Fatalf("MarshalBinary failed: %v", err)
			}

			// Unmarshal
			var header Header
			if err := header.UnmarshalBinary(data); err != nil {
				t.Fatalf("UnmarshalBinary failed: %v", err)
			}

			// Compare
			if header.Name != tc.header.Name {
				t.Errorf("Name mismatch: expected %q, got %q", tc.header.Name, header.Name)
			}
			if header.IsError != tc.header.IsError {
				t.Errorf("IsError mismatch: expected %v, got %v", tc.header.IsError, header.IsError)
			}
			if !bytes.Equal(header.Payload, tc.header.Payload) {
				t.Errorf("Payload mismatch: expected %v, got %v", tc.header.Payload, header.Payload)
			}
		})
	}
}

func TestHeader_MarshalBinary_EdgeCases(t *testing.T) {
	testCases := []struct {
		name   string
		header Header
	}{
		{
			name:   "Zero values",
			header: Header{},
		},
		{
			name: "Empty name",
			header: Header{
				Name:    "",
				IsError: true,
				Payload: []byte("error"),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := tc.header.MarshalBinary()
			if err != nil {
				t.Fatalf("MarshalBinary failed: %v", err)
			}

			var header Header
			if err := header.UnmarshalBinary(data); err != nil {
				t.Fatalf("UnmarshalBinary failed: %v", err)
			}

			if header.Name != tc.header.Name {
				t.Errorf("Name mismatch: expected %q, got %q", tc.header.Name, header.Name)
			}
			if header.IsError != tc.header.IsError {
				t.Errorf("IsError mismatch: expected %v, got %v", tc.header.IsError, header.IsError)
			}
			if !bytes.Equal(header.Payload, tc.header.Payload) {
				t.Errorf("Payload mismatch: expected %v, got %v", tc.header.Payload, header.Payload)
			}
		})
	}
}

func TestHeader_UnmarshalBinary_InvalidData(t *testing.T) {
	testCases := []struct {
		name string
		data []byte
	}{
		{
			name: "Empty data",
			data: []byte{},
		},
		{
			name: "Too short for name length",
			data: []byte{1, 2, 3},
		},
		{
			name: "Name length exceeds data",
			data: []byte{0, 0, 1, 0}, // name length = 256, but no data follows
		},
		{
			name: "Missing IsError byte",
			data: append([]byte{0, 0, 0, 5}, []byte("hello")...), // name length + name, but no IsError
		},
		{
			name: "Missing payload length",
			data: append(append([]byte{0, 0, 0, 5}, []byte("hello")...), 1), // name + IsError, but no payload length
		},
		{
			name: "Payload length exceeds data",
			data: append(append(append([]byte{0, 0, 0, 5}, []byte("hello")...), 1), []byte{0, 0, 1, 0}...), // payload length = 256, but no payload
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var header Header
			err := header.UnmarshalBinary(tc.data)
			if err == nil {
				t.Error("Expected error for invalid data, but got nil")
			}
		})
	}
}

func TestHeader_RoundTrip_Consistency(t *testing.T) {
	// Test with random data to ensure consistency
	header := Header{
		Name:    "test_service_123",
		IsError: true,
		Payload: []byte("This is a test payload with various characters: !@#$%^&*()_+-=[]{}|;':\",./<>?"),
	}

	// Marshal and unmarshal multiple times
	for i := 0; i < 10; i++ {
		data, err := header.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary failed on iteration %d: %v", i, err)
		}

		var newHeader Header
		if err := newHeader.UnmarshalBinary(data); err != nil {
			t.Fatalf("UnmarshalBinary failed on iteration %d: %v", i, err)
		}

		if newHeader.Name != header.Name {
			t.Errorf("Name mismatch on iteration %d: expected %q, got %q", i, header.Name, newHeader.Name)
		}
		if newHeader.IsError != header.IsError {
			t.Errorf("IsError mismatch on iteration %d: expected %v, got %v", i, header.IsError, newHeader.IsError)
		}
		if !bytes.Equal(newHeader.Payload, header.Payload) {
			t.Errorf("Payload mismatch on iteration %d", i)
		}

		// Use the new header for next iteration
		header = newHeader
	}
}

// Benchmark tests
func BenchmarkHeader_MarshalBinary(b *testing.B) {
	header := Header{
		Name:    "benchmark_service",
		IsError: false,
		Payload: make([]byte, 1024), // 1KB payload
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := header.MarshalBinary()
		if err != nil {
			b.Fatalf("MarshalBinary failed: %v", err)
		}
	}
}

func BenchmarkHeader_UnmarshalBinary(b *testing.B) {
	header := Header{
		Name:    "benchmark_service",
		IsError: false,
		Payload: make([]byte, 1024), // 1KB payload
	}

	data, err := header.MarshalBinary()
	if err != nil {
		b.Fatalf("MarshalBinary failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var h Header
		err := h.UnmarshalBinary(data)
		if err != nil {
			b.Fatalf("UnmarshalBinary failed: %v", err)
		}
	}
}

func BenchmarkHeader_MarshalUnmarshal(b *testing.B) {
	header := Header{
		Name:    "benchmark_service",
		IsError: false,
		Payload: make([]byte, 1024), // 1KB payload
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, err := header.MarshalBinary()
		if err != nil {
			b.Fatalf("MarshalBinary failed: %v", err)
		}

		var h Header
		err = h.UnmarshalBinary(data)
		if err != nil {
			b.Fatalf("UnmarshalBinary failed: %v", err)
		}
	}
}
