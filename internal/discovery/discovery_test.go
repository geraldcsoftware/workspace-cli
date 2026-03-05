package discovery

import "testing"

func TestMatchQuery(t *testing.T) {
	repos := []string{
		"/tmp/work/alpha-service",
		"/tmp/work/beta-service",
	}

	got, err := MatchQuery(repos, "alpha")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/tmp/work/alpha-service" {
		t.Fatalf("unexpected repo: %s", got)
	}
}

func TestMatchQueryAmbiguous(t *testing.T) {
	repos := []string{
		"/tmp/work/auth-service",
		"/tmp/work/auth-proxy",
	}
	_, err := MatchQuery(repos, "auth")
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
	if _, ok := err.(*AmbiguousMatchError); !ok {
		t.Fatalf("expected AmbiguousMatchError, got %T", err)
	}
}
