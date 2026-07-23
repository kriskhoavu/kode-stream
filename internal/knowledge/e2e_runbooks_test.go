package knowledge

import "testing"

func TestMatchesE2ESource(t *testing.T) {
	if !matchesE2ESource([]string{"plans/platform/PM-036/automation/README.md"}, []string{"plans/platform/PM-036/automation/README.md"}) {
		t.Fatal("expected exact source reference to match")
	}
	if matchesE2ESource([]string{"plans/platform/PM-029/automation/README.md"}, []string{"plans/platform/PM-036/automation/README.md"}) {
		t.Fatal("unexpected source reference match")
	}
}
