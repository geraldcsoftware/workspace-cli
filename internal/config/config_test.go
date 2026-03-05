package config

import "testing"

func TestExpandPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if got, want := expandPath("~/ws"), home+"/ws"; got != want {
		t.Fatalf("expandPath got %q want %q", got, want)
	}
}
