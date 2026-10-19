package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"debug/macho"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type artifact struct {
	arch   string
	name   string
	sha256 string
}

var stableVersion = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

func validVersion(version string) error {
	if !stableVersion.MatchString(version) {
		return fmt.Errorf("version must be a stable vX.Y.Z release: %q", version)
	}
	return nil
}

func ensureAbsent(path string) error {
	_, err := os.Lstat(path)
	if err == nil {
		return fmt.Errorf("release output already exists: %s", path)
	}
	if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func writeArchive(path string, binary, license, completion []byte) (string, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return "", err
	}
	gz := gzip.NewWriter(f)
	gz.ModTime = time.Unix(1, 0)
	gz.OS = 255
	tw := tar.NewWriter(gz)
	for _, entry := range []struct {
		name string
		mode int64
		data []byte
	}{{"dia", 0755, binary}, {"LICENSE", 0644, license}, {"_dia", 0644, completion}} {
		h := &tar.Header{Name: entry.name, Mode: entry.mode, Size: int64(len(entry.data)), ModTime: time.Unix(0, 0), Typeflag: tar.TypeReg, Format: tar.FormatUSTAR}
		if err := tw.WriteHeader(h); err != nil {
			f.Close()
			return "", err
		}
		if _, err := tw.Write(entry.data); err != nil {
			f.Close()
			return "", err
		}
	}
	if err := tw.Close(); err != nil {
		f.Close()
		return "", err
	}
	if err := gz.Close(); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

func formula(version string, artifacts []artifact) string {
	return fmt.Sprintf(`class DiaCli < Formula
  desc "Control Dia tabs, windows, and profiles from the terminal"
  homepage "https://github.com/Jermic/dia-cli"
  version "%s"
  license "MIT"
  depends_on :macos

  on_arm do
    url "https://github.com/Jermic/dia-cli/releases/download/%s/%s"
    sha256 "%s"
  end

  on_intel do
    url "https://github.com/Jermic/dia-cli/releases/download/%s/%s"
    sha256 "%s"
  end

  def install
    bin.install "dia"
    prefix.install "LICENSE"
    zsh_completion.install "_dia"
  end

  test do
    assert_match "dia #{version}", shell_output("#{bin}/dia --version")
    assert_match "dia <command>", shell_output("#{bin}/dia help")
    assert_path_exists zsh_completion/"_dia"
  end
end
`, strings.TrimPrefix(version, "v"), version, artifacts[0].name, artifacts[0].sha256, version, artifacts[1].name, artifacts[1].sha256)
}

func release(version, binariesDir string) error {
	if err := validVersion(version); err != nil {
		return err
	}
	status := exec.Command("git", "status", "--porcelain", "--untracked-files=all")
	output, err := status.CombinedOutput()
	if err != nil {
		return fmt.Errorf("check source status: %w: %s", err, output)
	}
	if len(output) != 0 {
		return fmt.Errorf("release requires a clean source tree: %s", output)
	}
	tags := exec.Command("git", "tag", "--points-at", "HEAD")
	output, err = tags.CombinedOutput()
	if err != nil {
		return fmt.Errorf("check source tag: %w: %s", err, output)
	}
	if !strings.Contains("\n"+string(output), "\n"+version+"\n") {
		return fmt.Errorf("HEAD must have tag %s", version)
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	dist := filepath.Join(root, "dist")
	target := filepath.Join(dist, version)
	if err := ensureAbsent(target); err != nil {
		return err
	}
	license, err := os.ReadFile("LICENSE")
	if err != nil {
		return err
	}
	completion, err := os.ReadFile("completions/_dia")
	if err != nil {
		return err
	}
	artifacts := []artifact{{arch: "arm64"}, {arch: "amd64"}}
	prebuilt := make(map[string][]byte)
	if binariesDir != "" {
		for _, a := range artifacts {
			data, err := readBinary(filepath.Join(binariesDir, a.arch, "dia"), a.arch)
			if err != nil {
				return err
			}
			prebuilt[a.arch] = data
		}
	}
	if err := os.MkdirAll(dist, 0755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(dist, ".dia-release-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	var checksums strings.Builder
	for i := range artifacts {
		a := &artifacts[i]
		a.name = fmt.Sprintf("dia-cli_%s_darwin_%s.tar.gz", strings.TrimPrefix(version, "v"), a.arch)
		data := prebuilt[a.arch]
		if binariesDir == "" {
			binary := filepath.Join(tmp, "dia-"+a.arch)
			cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w -X main.version="+strings.TrimPrefix(version, "v"), "-o", binary, ".")
			cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=darwin", "GOARCH="+a.arch)
			cmd.Dir = root
			if output, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("build %s: %w: %s", a.arch, err, output)
			}
			data, err = readBinary(binary, a.arch)
			if err != nil {
				return err
			}
			if err := os.Remove(binary); err != nil {
				return err
			}
		}
		a.sha256, err = writeArchive(filepath.Join(tmp, a.name), data, license, completion)
		if err != nil {
			return err
		}
		fmt.Fprintf(&checksums, "%s  %s\n", a.sha256, a.name)
	}
	if err := os.WriteFile(filepath.Join(tmp, "checksums.txt"), []byte(checksums.String()), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(tmp, "dia-cli.rb"), []byte(formula(version, artifacts)), 0644); err != nil {
		return err
	}
	if err := ensureAbsent(target); err != nil {
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		return err
	}
	fmt.Println(target)
	return nil
}

func readBinary(path, arch string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s binary: %w", arch, err)
	}
	binary, err := macho.NewFile(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("read %s binary: %w", arch, err)
	}
	defer binary.Close()
	want := macho.CpuArm64
	if arch == "amd64" {
		want = macho.CpuAmd64
	}
	if binary.Cpu != want || binary.Type != macho.TypeExec {
		return nil, fmt.Errorf("expected %s Mach-O executable: %s", arch, path)
	}
	return data, nil
}

func main() {
	binariesDir := flag.String("binaries-dir", "", "package verified native binaries from DIR/{arm64,amd64}/dia")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/release [--binaries-dir DIR] vX.Y.Z")
		os.Exit(2)
	}
	if err := release(flag.Arg(0), *binariesDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
