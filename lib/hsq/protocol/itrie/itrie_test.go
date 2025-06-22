package itrie_test

import (
	"math/rand/v2"
	"sync"
	"testing"

	"github.com/snowmerak/plugin.go/lib/hsq/protocol/itrie"
)

func TestITrie_InsertAndSearch(t *testing.T) {
	trie := itrie.New[int]()
	value := 42
	trie.Insert(0x1234567890abcdef, &value)
	result := trie.Search(0x1234567890abcdef)

	if result == nil {
		t.Fatal("Expected value, got nil")
	}

	if *result != value {
		t.Fatalf("Expected %d, got %d", value, *result)
	}
}

func TestITrie_SearchNonExistentKey(t *testing.T) {
	trie := itrie.New[int]()
	result := trie.Search(0x1234567890abcdef)
	if result != nil {
		t.Fatalf("Expected nil, got %d", *result)
	}
}

func TestITrie_InsertAndDelete(t *testing.T) {
	trie := itrie.New[int]()
	value := 42
	trie.Insert(0x1234567890abcdef, &value)
	trie.Delete(0x1234567890abcdef)
	result := trie.Search(0x1234567890abcdef)
	if result != nil {
		t.Fatalf("Expected nil, got %d", *result)
	}
}

func TestITrie_DeleteNonExistentKey(t *testing.T) {
	trie := itrie.New[int]()
	trie.Delete(0x1234567890abcdef)
	// No panic should occur
}

func TestITrie_InsertDuplicateKey(t *testing.T) {
	trie := itrie.New[int]()
	value1 := 42
	value2 := 84
	trie.Insert(0x1234567890abcdef, &value1)
	trie.Insert(0x1234567890abcdef, &value2)
	result := trie.Search(0x1234567890abcdef)
	if result == nil {
		t.Fatalf("Expected value, got nil")
	}
	if *result != value2 {
		t.Fatalf("Expected %d, got %d", value2, *result)
	}
}

func TestITrie_ConcurrentInsert(t *testing.T) {
	trie := itrie.New[int]()
	sm := sync.Map{}

	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		key := rand.Uint64()
		wg.Add(1)
		go func(i int) {
			trie.Insert(key, &i)
			wg.Done()
		}(i)
		wg.Add(1)
		go func(i int) {
			sm.Store(key, i)
			wg.Done()
		}(i)
	}

	wg.Wait()

	sm.Range(func(key, value any) bool {
		k := key.(uint64)
		v := value.(int)

		result := trie.Search(k)
		if result == nil {
			t.Fatalf("Expected value, got nil")
		}

		if *result != v {
			t.Fatalf("Expected %d, got %d", v, *result)
		}

		return true
	})
}

func TestITrie_ConcurrentDelete(t *testing.T) {
	trie := itrie.New[int]()
	sm := sync.Map{}

	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		key := rand.Uint64()
		trie.Insert(key, &i)
		sm.Store(key, i)
	}

	sm.Range(func(key, value any) bool {
		k := key.(uint64)
		wg.Add(1)
		go func() {
			trie.Delete(k)
			wg.Done()
		}()
		return true
	})

	wg.Wait()

	sm.Range(func(key, value any) bool {
		k := key.(uint64)
		result := trie.Search(k)
		if result != nil {
			t.Fatalf("Expected nil, got %d", *result)
		}
		return true
	})
}

func TestITrie_ConcurrentInsertAndDelete(t *testing.T) {
	trie := itrie.New[int]()
	sm := sync.Map{}

	var wg sync.WaitGroup
	for i := 0; i < 10000; i++ {
		key := rand.Uint64()
		wg.Add(1)
		go func(i int) {
			trie.Insert(key, &i)
			wg.Done()
		}(i)
		wg.Add(1)
		go func(i int) {
			sm.Store(key, i)
			wg.Done()
		}(i)
	}

	wg.Wait()

	sm.Range(func(key, value any) bool {
		k := key.(uint64)
		v := value.(int)

		result := trie.Search(k)
		if result == nil {
			t.Fatalf("Expected value, got nil")
		}

		if *result != v {
			t.Fatalf("Expected %d, got %d", v, *result)
		}

		return true
	})
}

func BenchmarkITrie_InsertAndDelete(b *testing.B) {
	trie := itrie.New[int]()
	wg := sync.WaitGroup{}
	for i := 0; i < b.N; i++ {
		key := rand.Uint64()
		wg.Add(2)
		go func() {
			trie.Insert(key, &i)
			wg.Done()
		}()
		go func() {
			trie.Delete(key)
			wg.Done()
		}()
	}
	wg.Wait()
}
