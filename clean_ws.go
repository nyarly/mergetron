package main

import "strings"

func cleanWS(doc string) string {
	lines := strings.Split(doc, "\n")
	if len(lines) < 2 {
		return doc
	}
	for len(lines[0]) == 0 {
		lines = lines[1:]
	}

	for {
		tryLines := make([]string, 0, len(lines))
		first := lines[0]

		indent := first[0]

		for idx := range lines {
			if len(lines[idx]) == 0 {
				tryLines = append(tryLines, lines[idx])
				continue
			}
			if indent != lines[idx][0] {
				return strings.Join(lines, "\n")
			}
			tryLines = append(tryLines, lines[idx][1:])
		}
		lines = tryLines
	}
}
