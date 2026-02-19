package buildpack

import "testing"

func TestUniqueSortedVersionsUsesSemverOrder(t *testing.T) {
	versions := []string{"1.9.0", "1.10.0", "1.2.0", "1.10.0"}
	got := uniqueSortedVersions(versions)

	want := []string{"1.2.0", "1.9.0", "1.10.0"}
	if len(got) != len(want) {
		t.Fatalf("len(got) = %d, want %d; got=%v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q (all=%v)", i, got[i], want[i], got)
		}
	}
}
