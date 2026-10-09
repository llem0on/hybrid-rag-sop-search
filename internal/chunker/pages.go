package chunker

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

type Page struct {
	Number int
	Text   string
}

var (
	rePageNumber   = regexp.MustCompile(`^\s*(halaman|page|hal)?\.?\s*:?\s*\d+\s*(of|/|dari)?\s*\d*\s*$`)
	reMultiSpace   = regexp.MustCompile(`[ \t]+`)
	reMultiNewline = regexp.MustCompile(`\n{3,}`)
)

func ExtractPages(ctx context.Context, pdfPath string) ([]Page, error) {
	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, "pdftotext", pdfPath, "-")
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("pdftotext: %w", err)
	}
	return SplitPages(out.String()), nil
}

func SplitPages(raw string) []Page {
	parts := strings.Split(raw, "\f")
	pages := make([]Page, 0, len(parts))
	for i, p := range parts {
		if strings.TrimSpace(p) != "" {
			pages = append(pages, Page{Number: i + 1, Text: p})
		}
	}
	return pages
}

func CleanPages(pages []Page) []Page {
	lineFreq := map[string]int{}
	for _, pg := range pages {
		seen := map[string]bool{}
		for _, line := range strings.Split(pg.Text, "\n") {
			t := strings.TrimSpace(line)
			if t != "" && !seen[t] {
				seen[t] = true
				lineFreq[t]++
			}
		}
	}

	threshold := max(len(pages)/2, 2)
	cleaned := make([]Page, 0, len(pages))
	for _, pg := range pages {
		var b strings.Builder
		for _, line := range strings.Split(pg.Text, "\n") {
			t := strings.TrimSpace(line)
			switch {
			case t == "":
				b.WriteString("\n")
			case rePageNumber.MatchString(strings.ToLower(t)):
			case lineFreq[t] >= threshold && len(t) < 80 && !isLikelyHeading(t):
			default:
				b.WriteString(reMultiSpace.ReplaceAllString(t, " "))
				b.WriteString("\n")
			}
		}
		text := reMultiNewline.ReplaceAllString(b.String(), "\n\n")
		cleaned = append(cleaned, Page{Number: pg.Number, Text: strings.TrimSpace(text)})
	}
	return cleaned
}
