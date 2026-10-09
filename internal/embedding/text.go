package embedding

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	DefaultDocInstruction   = "Represent the user's input."
	DefaultQueryInstruction = "Retrieve relevant documents for the query."
)

var rePurposeSentence = regexp.MustCompile(`[^.]{20,400}\.`)

func ChatTemplate(instruction, text string) string {
	return "<|im_start|>system\n" + strings.TrimSpace(instruction) + "\n<|im_end|>\n" +
		"<|im_start|>user\n" + text + "\n<|im_end|>\n" +
		"<|im_start|>assistant\n"
}

func DocumentText(title, section, purpose, content string) string {
	return fmt.Sprintf("---\nDocument Title: %s\nSection: %s\nDescription: %s\n---\n%s", title, section, purpose, content)
}

func FigureText(title, section, content string) string {
	return fmt.Sprintf("---\nDocument Title: %s\nSection: %s\n---\n%s", title, section, content)
}

func PurposeSentence(chunkContent string) string {
	lines := strings.Split(chunkContent, "\n")
	if len(lines) < 2 {
		return ""
	}
	body := strings.Join(strings.Fields(strings.Join(lines[1:], "\n")), " ")
	return strings.TrimSpace(rePurposeSentence.FindString(body))
}

func IsPurposeSection(section string) bool {
	s := strings.ToLower(section)
	return strings.Contains(s, "tujuan") || strings.Contains(s, "purpose") || strings.Contains(s, "objective")
}
