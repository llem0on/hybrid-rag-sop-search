package figures

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	minOCRChars    = 30
	ocrTimeout     = 30 * time.Second
	extractTimeout = 2 * time.Minute
)

type Figure struct {
	Page    int
	Path    string
	Text    string
	Section string
}

type pdfXML struct {
	Pages []struct {
		Number int `xml:"number,attr"`
		Height int `xml:"height,attr"`
		Width  int `xml:"width,attr"`
		Images []struct {
			Top    int    `xml:"top,attr"`
			Left   int    `xml:"left,attr"`
			Width  int    `xml:"width,attr"`
			Height int    `xml:"height,attr"`
			Src    string `xml:"src,attr"`
		} `xml:"image"`
		Texts []struct {
			Top   int    `xml:"top,attr"`
			Left  int    `xml:"left,attr"`
			Inner string `xml:",innerxml"`
		} `xml:"text"`
	} `xml:"page"`
}

var (
	reTag        = regexp.MustCompile(`<[^>]+>`)
	reBadXMLChar = regexp.MustCompile("[\x00-\x08\x0B\x0C\x0E-\x1F]")
	htmlEntities = strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", "\"", "&#39;", "'")
)

func Extract(ctx context.Context, pdfPath, outDir, prefix, lang string) ([]Figure, error) {
	work, err := os.MkdirTemp("", "figures-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(work)

	xctx, cancel := context.WithTimeout(ctx, extractTimeout)
	defer cancel()
	base := filepath.Join(work, "doc")
	if out, err := exec.CommandContext(xctx, "pdftohtml", "-xml", "-q", "-nodrm", "-fmt", "png", pdfPath, base).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("pdftohtml: %w: %s", err, bytes.TrimSpace(out))
	}
	raw, err := os.ReadFile(base + ".xml")
	if err != nil {
		return nil, err
	}
	var doc pdfXML
	if err := xml.Unmarshal(reBadXMLChar.ReplaceAll(raw, nil), &doc); err != nil {
		return nil, fmt.Errorf("parse pdftohtml xml: %w", err)
	}

	lines := map[int][]Line{}
	for _, p := range doc.Pages {
		cells := make([]textCell, 0, len(p.Texts))
		for _, t := range p.Texts {
			if text := strings.TrimSpace(htmlEntities.Replace(reTag.ReplaceAllString(t.Inner, ""))); text != "" {
				cells = append(cells, textCell{top: t.Top, left: t.Left, text: text})
			}
		}
		lines[p.Number] = groupLines(cells, p.Height)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	var out []Figure
	seen := map[string]bool{}
	for _, p := range doc.Pages {
		for _, im := range p.Images {
			if p.Width > 0 && p.Height > 0 && im.Width*10 >= p.Width*9 && im.Height*10 >= p.Height*9 {
				continue
			}
			key := fmt.Sprintf("%d:%d:%d:%d", im.Top, im.Left, im.Width, im.Height)
			if seen[key] {
				continue
			}
			seen[key] = true

			src := im.Src
			if !filepath.IsAbs(src) {
				src = filepath.Join(work, filepath.Base(src))
			}
			text := ocr(ctx, src, lang)
			if len([]rune(text)) < minOCRChars {
				continue
			}
			name := fmt.Sprintf("%s-p%d-%d.png", prefix, p.Number, len(out)+1)
			if err := copyFile(src, filepath.Join(outDir, name)); err != nil {
				return nil, err
			}
			heading, caption := Context(lines, p.Number, im.Top, im.Height)
			out = append(out, Figure{Page: p.Number, Path: name, Text: text, Section: SectionTitle(p.Number, heading, caption)})
		}
	}
	return out, nil
}

func ocr(ctx context.Context, imagePath, lang string) string {
	octx, cancel := context.WithTimeout(ctx, ocrTimeout)
	defer cancel()
	out, err := exec.CommandContext(octx, "tesseract", imagePath, "stdout", "-l", lang).Output()
	if err != nil {
		return ""
	}
	return strings.Join(strings.Fields(string(out)), " ")
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
