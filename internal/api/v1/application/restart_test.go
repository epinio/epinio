package application

import "testing"

func TestComputeRestartImageURLReplacesTag(t *testing.T) {
	got := computeRestartImageURL(
		"192.168.49.2:30500/apps/workspace-sample:oldstage",
		"newstage",
		"workspace",
		"sample",
		"",
	)

	expected := "192.168.49.2:30500/apps/workspace-sample:newstage"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestComputeRestartImageURLReconstructsWhenMalformed(t *testing.T) {
	got := computeRestartImageURL(
		"c244aea3d59859ad",
		"c244aea3d59859ad",
		"workspace",
		"sdsdsxzxasdwsd",
		"192.168.49.2:30500/apps",
	)

	expected := "192.168.49.2:30500/apps/workspace-sdsdsxzxasdwsd:c244aea3d59859ad"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}
