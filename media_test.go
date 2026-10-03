package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDownload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.Repeat("x", 100)))
	}))
	defer srv.Close()

	dir := t.TempDir()
	f, err := newMediaFetcher(dir, 100)
	if err != nil {
		t.Fatal(err)
	}

	rel := filepath.Join("2026", "10", "03", "c", "m", "a-file.png")
	err = f.download(srv.URL, rel)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "ready", rel))
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 100 {
		t.Fatalf("got %d bytes, want 100", len(b))
	}

	f.maxBytes = 99
	err = f.download(srv.URL, "too-big")
	if err != errTooLarge {
		t.Fatalf("got err %v, want errTooLarge", err)
	}
	_, err = os.Stat(filepath.Join(dir, "ready", "too-big"))
	if !os.IsNotExist(err) {
		t.Fatalf("oversized file was written: %v", err)
	}

	tmps, err := os.ReadDir(filepath.Join(dir, "tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(tmps) != 0 {
		t.Fatalf("tmp dir not cleaned up: %v", tmps)
	}
}

func TestSafeName(t *testing.T) {
	cases := map[string]string{
		"photo.png":      "photo.png",
		"../../etc/pass": ".._.._etc_pass",
		"..":             "_",
		"":               "_",
		"a\\b":           "a_b",
	}
	for in, want := range cases {
		got := safeName(in)
		if got != want {
			t.Errorf("safeName(%q) = %q, want %q", in, got, want)
		}
	}
	if got := safeName(strings.Repeat("é", 150)); len(got) > 200 {
		t.Errorf("long name not truncated: %d bytes", len(got))
	}
}
