package idgen

import (
	"strings"
	"sync"
	"testing"
)

// TestIDsAreUnique pins uniqueness. A collision here means two runs believe they
// own the same state.
func TestIDsAreUnique(t *testing.T) {
	generator := New("run")

	const count = 20000
	seen := make(map[string]bool, count)
	for i := 0; i < count; i++ {
		id := generator.NewID()
		if id == "" {
			t.Fatal("an id must never be empty")
		}
		if seen[id] {
			t.Fatalf("duplicate id %q after %d draws", id, i)
		}
		seen[id] = true
	}
}

func TestIDCarriesThePrefix(t *testing.T) {
	generator := New("tag")
	if id := generator.NewID(); !strings.HasPrefix(id, "tag-") {
		t.Errorf("id = %q, want a tag- prefix", id)
	}
	// An empty prefix must still produce a usable, non-empty id rather than "-".
	if id := New("").NewID(); id == "" || id == "-" {
		t.Errorf("id = %q, want something usable", id)
	}
}

// TestConcurrentUseIsSafe covers the real access pattern: several goroutines
// starting runs at once.
func TestConcurrentUseIsSafe(t *testing.T) {
	generator := New("run")

	var (
		mu   sync.Mutex
		seen = map[string]bool{}
		wg   sync.WaitGroup
	)

	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				id := generator.NewID()
				mu.Lock()
				if seen[id] {
					t.Errorf("duplicate id %q across goroutines", id)
				}
				seen[id] = true
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if len(seen) != 8*500 {
		t.Errorf("collected %d ids, want %d", len(seen), 8*500)
	}
}
