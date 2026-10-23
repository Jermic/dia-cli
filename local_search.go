package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type HistoryItem struct {
	ID            int64  `json:"id"`
	Title         string `json:"title"`
	URL           string `json:"url"`
	LastVisitedAt string `json:"lastVisitedAt"`
}

type BookmarkItem struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	URL    string `json:"url"`
	Folder string `json:"folder"`
}

func isLocalSearch(name string) bool { return name == "history" || name == "bookmarks" }

func parseLocalSearch(name string, args []string) (Command, error) {
	c := Command{Name: name, Sort: "title"}
	literal := false
	for _, arg := range args {
		if !literal {
			switch arg {
			case "--":
				literal = true
				continue
			case "-h", "--help":
				return Command{Name: "help", Args: []string{name}}, nil
			case "--json":
				if c.JSON {
					return c, errors.New("--json was repeated")
				}
				c.JSON = true
				continue
			}
			if strings.HasPrefix(arg, "-") {
				return c, fmt.Errorf("unexpected argument: %s", arg)
			}
		}
		c.Args = append(c.Args, arg)
	}
	if len(c.Args) != 1 || strings.TrimSpace(c.Args[0]) == "" {
		return c, fmt.Errorf("usage: dia %s <text> [--json] requires one non-empty search argument", name)
	}
	if err := keyword(c.Args[0]); err != nil {
		return c, err
	}
	return c, nil
}

func localSource(name string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	root := filepath.Join(home, "Library/Application Support/Dia/User Data")
	var state struct {
		Profile struct {
			LastUsed string `json:"last_used"`
		} `json:"profile"`
	}
	data, err := os.ReadFile(filepath.Join(root, "Local State"))
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("read Dia Local State: %w", err)
	}
	if err == nil {
		if err := json.Unmarshal(data, &state); err != nil {
			return "", fmt.Errorf("invalid Dia Local State: %w", err)
		}
	}
	dir := state.Profile.LastUsed
	if dir == "" {
		dir = "Default"
	}
	if dir == "." || dir == ".." || strings.ContainsAny(dir, "/\\") || filepath.IsAbs(dir) {
		return "", errors.New("invalid Dia disk profile directory")
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("locate Dia user data: %w", err)
	}
	path, err := filepath.EvalSymlinks(filepath.Join(root, dir, name))
	if err != nil {
		return "", fmt.Errorf("locate Dia %s: %w", name, err)
	}
	rel, err := filepath.Rel(canonicalRoot, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", errors.New("Dia data file escapes user data directory")
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("Dia %s is not a regular file", name)
	}
	return path, nil
}

func localSearch(c Command) (Result, error) {
	source := "History"
	if c.Name == "bookmarks" {
		source = "Bookmarks"
	}
	path, err := localSource(source)
	if err != nil {
		return Result{}, err
	}
	query := strings.ToLower(c.Args[0])
	matches := func(title, url string) bool {
		return strings.Contains(strings.ToLower(title), query) || strings.Contains(strings.ToLower(url), query)
	}
	r := Result{}
	var output strings.Builder
	if c.Name == "history" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "/usr/bin/sqlite3", "-readonly", "-batch", "-json", "-init", "/dev/null", "-cmd", ".timeout 2000", path, `SELECT id, COALESCE(title,'') AS title, url, last_visit_time FROM urls ORDER BY last_visit_time DESC, id DESC;`)
		var diagnostics bytes.Buffer
		cmd.Stderr = &diagnostics
		data, err := cmd.Output()
		if ctx.Err() != nil {
			return r, fmt.Errorf("read Dia History: %w", ctx.Err())
		}
		if err != nil {
			return r, fmt.Errorf("read Dia History: %w%s", err, diagnosticSuffix(safeText(diagnostics.String())))
		}
		var rows []struct {
			ID            int64  `json:"id"`
			Title         string `json:"title"`
			URL           string `json:"url"`
			LastVisitTime int64  `json:"last_visit_time"`
		}
		if len(bytes.TrimSpace(data)) > 0 {
			if err := json.Unmarshal(data, &rows); err != nil {
				return r, fmt.Errorf("invalid Dia History result: %w", err)
			}
		}
		r.History = make([]HistoryItem, 0)
		output.WriteString("ID\tLAST VISITED\tTITLE\tURL")
		for _, row := range rows {
			if !matches(row.Title, row.URL) {
				continue
			}
			visited := ""
			if row.LastVisitTime > 0 {
				visited = time.UnixMicro(row.LastVisitTime - 11644473600000000).UTC().Format(time.RFC3339Nano)
			}
			item := HistoryItem{row.ID, row.Title, row.URL, visited}
			r.History = append(r.History, item)
			fmt.Fprintf(&output, "\n%d\t%s\t%s\t%s", item.ID, item.LastVisitedAt, safeText(item.Title), safeText(item.URL))
			if len(r.History) == 100 {
				break
			}
		}
	} else {
		type node struct {
			ID       string            `json:"id"`
			Name     string            `json:"name"`
			Type     string            `json:"type"`
			URL      string            `json:"url"`
			Children []json.RawMessage `json:"children"`
		}
		var document struct {
			Roots map[string]json.RawMessage `json:"roots"`
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return r, fmt.Errorf("read Dia Bookmarks: %w", err)
		}
		if err := json.Unmarshal(data, &document); err != nil {
			return r, fmt.Errorf("invalid Dia Bookmarks: %w", err)
		}
		if document.Roots == nil {
			return r, errors.New("invalid Dia Bookmarks: missing roots")
		}
		r.Bookmarks = make([]BookmarkItem, 0)
		output.WriteString("ID\tFOLDER\tTITLE\tURL")
		var walk func(json.RawMessage, []string) error
		walk = func(raw json.RawMessage, folders []string) error {
			var n node
			if err := json.Unmarshal(raw, &n); err != nil {
				return err
			}
			if n.Type == "url" {
				if matches(n.Name, n.URL) {
					item := BookmarkItem{n.ID, n.Name, n.URL, strings.Join(folders, " / ")}
					r.Bookmarks = append(r.Bookmarks, item)
					fmt.Fprintf(&output, "\n%s\t%s\t%s\t%s", safeText(item.ID), safeText(item.Folder), safeText(item.Title), safeText(item.URL))
				}
				return nil
			}
			if n.Name != "" {
				folders = append(append([]string(nil), folders...), n.Name)
			}
			for _, child := range n.Children {
				if err := walk(child, folders); err != nil {
					return err
				}
			}
			return nil
		}
		keys := make([]string, 0, len(document.Roots))
		for k := range document.Roots {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if err := walk(document.Roots[key], nil); err != nil {
				return r, fmt.Errorf("invalid Dia bookmark node: %w", err)
			}
		}
	}
	r.Output = output.String()
	return r, nil
}
