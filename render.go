package main

import (
	"os"
	"strconv"
	"strings"
	"syscall"
	"unicode"
	"unsafe"
)

func width(text string) int {
	n := 0
	for _, r := range text {
		if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Cf, r) {
			continue
		}
		if r >= 0x1100 && (r <= 0x115f || r >= 0x2329 && r <= 0x232a || r >= 0x2e80 && r <= 0xa4cf || r >= 0xac00 && r <= 0xd7a3 || r >= 0xf900 && r <= 0xfaff || r >= 0xfe10 && r <= 0xfe6f || r >= 0xff00 && r <= 0xff60 || r >= 0xffe0 && r <= 0xffe6 || r >= 0x1f300) {
			n += 2
		} else {
			n++
		}
	}
	return n
}

func cell(text string, columns int) string {
	if width(text) > columns {
		var b strings.Builder
		for _, r := range text {
			if width(b.String()+string(r)) >= columns {
				break
			}
			b.WriteRune(r)
		}
		text = b.String() + "…"
	}
	return text + strings.Repeat(" ", max(0, columns-width(text)))
}

func link(url, text string) string {
	if !webURL(url) {
		return text
	}
	return "\x1b]8;;" + url + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}

func renderProfiles(tsv string) string {
	if tsv == "" {
		return ""
	}
	lines := strings.Split(tsv, "\n")
	rows := make([][]string, len(lines))
	widths := []int{}
	for i, line := range lines {
		rows[i] = strings.Split(line, "\t")
		for j, value := range rows[i] {
			if j == len(widths) {
				widths = append(widths, 0)
			}
			widths[j] = max(widths[j], width(value))
		}
	}
	var b strings.Builder
	for _, row := range rows {
		for j, value := range row {
			if j > 0 {
				b.WriteString("  ")
			}
			if j == 1 || j == 3 {
				b.WriteString(strings.Repeat(" ", widths[j]-width(value)))
			}
			b.WriteString(value)
			if j < len(row)-1 && j != 1 && j != 3 {
				b.WriteString(strings.Repeat(" ", widths[j]-width(value)))
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func renderTabs(tsv string, detail bool) string {
	lines := strings.Split(tsv, "\n")
	if len(lines) == 0 {
		return ""
	}
	columns := terminalColumns(os.Stdout)
	if columns == 0 {
		columns = 80
	}
	if columns < 16 {
		columns = 16
	}
	count := len(lines) - 1
	numberWidth := len(strconv.Itoa(count))
	titleWidth := columns - numberWidth - 3
	if detail {
		titleWidth = (columns - numberWidth - 5) * 2 / 5
		if titleWidth > 48 {
			titleWidth = 48
		}
	}
	var b strings.Builder
	b.WriteString("\x1b[1;33m" + cell("#", numberWidth) + "\x1b[0m  \x1b[1;36m" + cell("TITLE", titleWidth) + "\x1b[0m")
	if detail {
		b.WriteString("  \x1b[1;34mURL\x1b[0m")
	}
	b.WriteByte('\n')
	for i, line := range lines[1:] {
		fields := strings.Split(line, "\t")
		if len(fields) < 4 {
			continue
		}
		title, url := fields[len(fields)-2], fields[len(fields)-1]
		if title == "" {
			title = "(untitled)"
		}
		b.WriteString("\x1b[33m" + strings.Repeat(" ", max(0, numberWidth-len(strconv.Itoa(i+1)))) + strconv.Itoa(i+1) + "\x1b[0m  \x1b[1m")
		b.WriteString(link(url, cell(title, titleWidth)) + "\x1b[0m")
		if detail {
			b.WriteString("  \x1b[2;34m" + link(url, url) + "\x1b[0m")
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func terminalColumns(file *os.File) int {
	var size struct{ Rows, Cols, Xpixel, Ypixel uint16 }
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&size)))
	if errno == 0 && size.Cols > 0 {
		return int(size.Cols)
	}
	columns, _ := strconv.Atoi(os.Getenv("COLUMNS"))
	return columns
}
