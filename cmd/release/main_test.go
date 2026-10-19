package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"debug/macho"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
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
	binary, license, completion := []byte("binary fixture"), []byte("license fixture"), []byte("completion fixture")
	sum, err := writeArchive(path, binary, license, completion)
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
	}{{"dia", 0755, string(binary)}, {"LICENSE", 0644, string(license)}, {"_dia", 0644, string(completion)}} {
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
	if _, err := writeArchive(second, binary, license, completion); err != nil {
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
	if !strings.Contains(formula, `bin.install "dia"`) || !strings.Contains(formula, `prefix.install "LICENSE"`) || !strings.Contains(formula, `zsh_completion.install "_dia"`) || !strings.Contains(formula, `assert_path_exists zsh_completion/"_dia"`) || !strings.Contains(formula, `shell_output("#{bin}/dia --version")`) || !strings.Contains(formula, `shell_output("#{bin}/dia help")`) {
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

func machoFixture(t *testing.T, path string, cpu macho.Cpu) []byte {
	t.Helper()
	var data bytes.Buffer
	for _, value := range []uint32{macho.Magic64, uint32(cpu), 0, uint32(macho.TypeExec), 0, 0, 0, 0} {
		if err := binary.Write(&data, binary.LittleEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data.Bytes(), 0755); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func TestPrebuiltBinaryArchitecture(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dia")
	data := machoFixture(t, path, macho.CpuArm64)
	got, err := readBinary(path, "arm64")
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("arm64 payload changed: %v", err)
	}
	if _, err := readBinary(path, "amd64"); err == nil {
		t.Fatal("accepted wrong architecture")
	}
	if _, err := readBinary(path+"-missing", "arm64"); err == nil {
		t.Fatal("accepted missing binary")
	}
	binary.LittleEndian.PutUint32(data[12:16], uint32(macho.TypeObj))
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := readBinary(path, "arm64"); err == nil {
		t.Fatal("accepted non-executable Mach-O")
	}
	if err := os.WriteFile(path, []byte("not Mach-O"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := readBinary(path, "arm64"); err == nil {
		t.Fatal("accepted malformed binary")
	}
}

func TestReleaseReusesPrebuiltBytesAndPreservesGuards(t *testing.T) {
	root := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatal(err)
		}
	})
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, output)
		}
	}
	git("init", "--quiet")
	for name, contents := range map[string]string{"LICENSE": "license", "completions/_dia": "completion", ".gitignore": "/dist/\n"} {
		if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(contents), 0644); err != nil {
			t.Fatal(err)
		}
	}
	git("add", ".")
	git("-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "fixture")
	git("tag", "v1.2.3")
	dir := filepath.Join(root, "dist", "binaries")
	arm := machoFixture(t, filepath.Join(dir, "arm64", "dia"), macho.CpuArm64)
	if err := release("v1.2.3", dir); err == nil {
		t.Fatal("accepted missing amd64 binary")
	}
	if _, err := os.Stat("dist/v1.2.3"); !os.IsNotExist(err) {
		t.Fatal("failed release left output")
	}
	machoFixture(t, filepath.Join(dir, "amd64", "dia"), macho.CpuArm64)
	if err := release("v1.2.3", dir); err == nil {
		t.Fatal("accepted wrong amd64 architecture")
	}
	if _, err := os.Stat("dist/v1.2.3"); !os.IsNotExist(err) {
		t.Fatal("wrong-architecture release left output")
	}
	amd := machoFixture(t, filepath.Join(dir, "amd64", "dia"), macho.CpuAmd64)
	if err := release("v1.2.4", dir); err == nil {
		t.Fatal("accepted absent tag")
	}
	if err := os.WriteFile("LICENSE", []byte("dirty"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := release("v1.2.3", dir); err == nil {
		t.Fatal("accepted dirty source")
	}
	git("checkout", "--", "LICENSE")
	if err := release("v1.2.3", dir); err != nil {
		t.Fatal(err)
	}
	for arch, want := range map[string][]byte{"arm64": arm, "amd64": amd} {
		path := filepath.Join("dist", "v1.2.3", "dia-cli_1.2.3_darwin_"+arch+".tar.gz")
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		gz, err := gzip.NewReader(file)
		if err != nil {
			t.Fatal(err)
		}
		reader := tar.NewReader(gz)
		header, err := reader.Next()
		if err != nil || header.Name != "dia" {
			t.Fatalf("binary entry: %v", err)
		}
		got, err := io.ReadAll(reader)
		gz.Close()
		file.Close()
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("%s verified bytes changed: %v", arch, err)
		}
	}
	before, err := os.ReadFile("dist/v1.2.3/checksums.txt")
	if err != nil {
		t.Fatal(err)
	}
	if err := release("v1.2.3", dir); err == nil {
		t.Fatal("overwrote existing output")
	}
	after, err := os.ReadFile("dist/v1.2.3/checksums.txt")
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("existing output changed")
	}
}
