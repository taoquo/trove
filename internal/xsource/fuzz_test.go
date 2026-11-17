package xsource

import (
	"os"
	"path/filepath"
	"testing"
)

// FuzzParsePage feeds arbitrary bytes through the two steps a response takes: the
// JSON decode and the page parser.
//
// The parser reads a payload X composed, and a payload that is malformed, truncated
// or hostile must produce an error, never a panic: the bytes come from the network.
// The corpus starts with the frozen fixtures in testdata/xsource, so the fuzzer
// begins from pages this app has actually parsed.
func FuzzParsePage(f *testing.F) {
	seeds, err := filepath.Glob(filepath.Join("..", "..", "testdata", "xsource", "*.json"))
	if err != nil {
		f.Fatalf("glob the corpus: %v", err)
	}
	for _, seed := range seeds {
		raw, err := os.ReadFile(seed)
		if err != nil {
			f.Fatalf("read %s: %v", seed, err)
		}
		f.Add(raw)
	}
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"data":{"bookmark_timeline_v2":{"timeline":{"instructions":[]}}}}`))

	f.Fuzz(func(t *testing.T, raw []byte) {
		payload, err := decodePayload(raw)
		if err != nil {
			return
		}
		// A rejected shape is an answer, not a failure; a panic is the failure.
		_, _ = ParseBookmarkEntries(payload, ParseOptions{SeenAt: testTime()})
	})
}
