package main

import "testing"

func TestGithubAnnotation(t *testing.T) {
	got := githubAnnotation("error", "screens/a,b:c/screen.yaml", "50% broken\nnext line")
	want := "::error file=screens/a%2Cb%3Ac/screen.yaml,title=munin lint::50%25 broken%0Anext line"
	if got != want {
		t.Errorf("githubAnnotation() = %q, want %q", got, want)
	}
}
