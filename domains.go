package main

import (
	"fmt"
	"net/netip"
	"net/url"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/net/publicsuffix"
)

type DomainCount struct {
	Domain string
	Count  int
}

func domainCounts(urls []string, subdomains bool) []DomainCount {
	counts := make(map[string]int)
	for _, raw := range urls {
		host := "(no domain)"
		u, err := url.Parse(raw)
		if err == nil && u.Scheme != "" && u.Hostname() != "" && !strings.ContainsFunc(raw, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
			candidate := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
			if address, err := netip.ParseAddr(candidate); err == nil {
				host = address.String()
			} else if validHostname(candidate) {
				host = candidate
				if !subdomains {
					if root, err := publicsuffix.EffectiveTLDPlusOne(host); err == nil {
						host = root
					}
				}
			}
		}
		counts[host]++
	}
	rows := make([]DomainCount, 0, len(counts))
	for host, count := range counts {
		rows = append(rows, DomainCount{host, count})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Count != rows[j].Count {
			return rows[i].Count > rows[j].Count
		}
		return rows[i].Domain < rows[j].Domain
	})
	return rows
}

func validHostname(host string) bool {
	if host == "" || len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' {
				return false
			}
		}
	}
	return true
}

func domainsOutput(rows []DomainCount) string {
	var output strings.Builder
	output.WriteString("DOMAIN\tCOUNT")
	for _, row := range rows {
		fmt.Fprintf(&output, "\n%s\t%d", row.Domain, row.Count)
	}
	return output.String()
}
