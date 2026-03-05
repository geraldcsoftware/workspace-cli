package workspace

import "testing"

func TestParsePorcelain(t *testing.T) {
	modified, staged, untracked := parsePorcelain(" M tracked.txt\nA  staged.txt\n?? new.txt\n")
	if modified != 1 {
		t.Fatalf("modified=%d, want 1", modified)
	}
	if staged != 1 {
		t.Fatalf("staged=%d, want 1", staged)
	}
	if untracked != 1 {
		t.Fatalf("untracked=%d, want 1", untracked)
	}
}
