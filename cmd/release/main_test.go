package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVersionValidation(t *testing.T) {
	for _, valid := range []string{"v0.1.0", "v12.345.6"} {
		if err := validVersion(valid); err != nil {
			t.Errorf("rejected %q: %v", valid, err)
		}
	}
	for _, invalid := range []string{"", "0.1.0", "v1.2", "v01.2.3", "v1.02.3", "v1.2.03", "v1.2.3-rc.1", "v1.2.3+build", "v1.2.3/../../x", " v1.2.3", "v1.2.3\n"} {
		if err := validVersion(invalid); err == nil {
			t.Errorf("accepted %q", invalid)
		}
	}
}

func TestArchiveContentsAndMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "asset.tar.gz")
	binary, license := []byte("binary fixture"), []byte("license fixture")
	sum, err := writeArchive(path, binary, license)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if sum != fmt.Sprintf("%x", sha256.Sum256(data)) {
		t.Fatal("checksum does not match archive bytes")
	}
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	if !gz.ModTime.Equal(time.Unix(1, 0)) {
		t.Fatalf("gzip timestamp: %s", gz.ModTime)
	}
	tr := tar.NewReader(gz)
	for _, want := range []struct {
		name string
		mode int64
		data string
	}{{"dia", 0755, string(binary)}, {"LICENSE", 0644, string(license)}} {
		h, err := tr.Next()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(tr)
		if err != nil || h.Name != want.name || h.Mode != want.mode || string(body) != want.data || !h.ModTime.Equal(time.Unix(0, 0)) {
			t.Fatalf("entry: %+v %q %v", h, body, err)
		}
	}
	if _, err := tr.Next(); err != io.EOF {
		t.Fatalf("extra archive entry: %v", err)
	}
	second := filepath.Join(t.TempDir(), "asset.tar.gz")
	if _, err := writeArchive(second, binary, license); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(second)
	if !bytes.Equal(again, data) {
		t.Fatal("same payload produced different archive")
	}
}

func TestFormulaBindsEachArchitectureToItsChecksum(t *testing.T) {
	artifacts := []artifact{
		{arch: "arm64", name: "dia-cli_0.1.0_darwin_arm64.tar.gz", sha256: strings.Repeat("a", 64)},
		{arch: "amd64", name: "dia-cli_0.1.0_darwin_amd64.tar.gz", sha256: strings.Repeat("b", 64)},
	}
	formula := formula("v0.1.0", artifacts)
	for _, a := range artifacts {
		block := "on_arm do"
		if a.arch == "amd64" {
			block = "on_intel do"
		}
		want := fmt.Sprintf("%s\n    url \"https://github.com/Jermic/dia-cli/releases/download/v0.1.0/%s\"\n    sha256 \"%s\"", block, a.name, a.sha256)
		if !strings.Contains(formula, want) {
			t.Errorf("missing matching %s URL and checksum", a.arch)
		}
	}
	if !strings.Contains(formula, `bin.install "dia"`) || !strings.Contains(formula, `prefix.install "LICENSE"`) || !strings.Contains(formula, `shell_output("#{bin}/dia --version")`) || !strings.Contains(formula, `shell_output("#{bin}/dia help")`) {
		t.Fatal("formula install or local test missing")
	}
}

func TestExistingReleaseDirectoryIsNotOverwritten(t *testing.T) {
	dist := t.TempDir()
	target := filepath.Join(dist, "v0.1.0")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(target, "marker")
	if err := os.WriteFile(marker, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ensureAbsent(target); err == nil {
		t.Fatal("existing release accepted")
	}
	if got, err := os.ReadFile(marker); err != nil || string(got) != "keep" {
		t.Fatal("existing release changed")
	}
}
