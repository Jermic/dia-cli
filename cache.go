package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

type rowCache struct {
	path string
	err  error
}

func isTerminal(file *os.File) bool {
	var state syscall.Termios
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), syscall.TIOCGETA, uintptr(unsafe.Pointer(&state)))
	return errno == 0
}

func cacheKey(tty string, parent int, started string) string {
	key := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%s", tty, parent, started)))
	return fmt.Sprintf("%x.json", key)
}

func newRowCache() *rowCache {
	cache := &rowCache{}
	parent := os.Getppid()
	identity, err := exec.Command("ps", "-o", "tty=", "-o", "lstart=", "-p", strconv.Itoa(parent)).Output()
	fields := strings.Fields(string(identity))
	if err != nil || len(fields) < 6 || fields[0] == "??" {
		cache.err = errors.New("cannot identify parent shell for row cache")
		return cache
	}
	dir := filepath.Join(os.TempDir(), fmt.Sprintf("dia-cli-%d", os.Getuid()))
	if err := os.Mkdir(dir, 0700); err != nil && !os.IsExist(err) {
		cache.err = err
		return cache
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 || info.Sys().(*syscall.Stat_t).Uid != uint32(os.Getuid()) {
		cache.err = errors.New("unsafe Dia row cache directory")
		return cache
	}
	cache.path = filepath.Join(dir, cacheKey(fields[0], parent, strings.Join(fields[1:], " ")))
	return cache
}

func (c *rowCache) save(rows []TabRef) error {
	if c.err != nil {
		return c.err
	}
	data, err := json.Marshal(rows)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(c.path), ".rows-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if err := file.Chmod(0600); err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), c.path)
}

func (c *rowCache) lookup(row string) (TabRef, error) {
	if c.err != nil {
		return TabRef{}, c.err
	}
	info, err := os.Lstat(c.path)
	if err == nil && (info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0600) {
		return TabRef{}, errors.New("unsafe saved tab list permissions")
	}
	data, err := os.ReadFile(c.path)
	if os.IsNotExist(err) {
		return TabRef{}, errors.New("no saved tab list. Run dia tabs or dia find in this terminal first")
	}
	if err != nil {
		return TabRef{}, err
	}
	var rows []TabRef
	if err := json.Unmarshal(data, &rows); err != nil {
		return TabRef{}, fmt.Errorf("invalid saved tab list: %w", err)
	}
	n, _ := strconv.Atoi(row)
	if n < 1 || n > len(rows) {
		return TabRef{}, errors.New("row is outside the saved list. Refresh with dia tabs or dia find")
	}
	return rows[n-1], nil
}
