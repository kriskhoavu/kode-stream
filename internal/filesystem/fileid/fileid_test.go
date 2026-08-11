package fileid

import "testing"

func TestIDsRoundTripWithoutCollisions(t *testing.T) {
	paths := []string{
		"a/b.md", "a__b.md", "a.b/c_d.md", "a_b/c.d.md", "unicode/世界.md", "Case/FILE.MD",
		" leading.md", "trailing .md ", "literal\\backslash.md", "directory/literal\\backslash.md",
	}
	seen := map[string]string{}
	for _, value := range paths {
		id := Encode(value)
		if previous, exists := seen[id]; exists {
			t.Fatalf("%q and %q collided as %q", previous, value, id)
		}
		seen[id] = value
		decoded, err := Decode(id)
		if err != nil || decoded != value {
			t.Fatalf("round trip %q: decoded=%q err=%v", value, decoded, err)
		}
	}
}

func TestDecodeRejectsLegacyLossyIDs(t *testing.T) {
	if _, err := Decode("a__b_md"); err == nil {
		t.Fatal("expected legacy ID rejection")
	}
}
