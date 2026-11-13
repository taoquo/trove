package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestZZCardLayout(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post about compilers", clockBase)
	shell.Reload(context.Background())
	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	for _, s := range []string{"@ada", "a post about compilers"} {
		if r, ok := tester.Find(s); ok {
			t.Logf("%-28q rect=%+v", s, r)
		}
	}
}
