package netproxy

import "testing"

func TestZZDebugParse(t *testing.T) {
	out := `<dictionary> { HTTPSEnable : 1
  HTTPSPort : 7890
  HTTPSProxy : user:secret@10.0.0.5 }`
	t.Logf("config = %+v", ParseScutil(out))
}
