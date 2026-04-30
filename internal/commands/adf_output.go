package commands

import (
	"fmt"
	"strings"

	apperr "github.com/major/jira-agent/internal/errors"
)

const (
	descriptionOutputFormatADF      = "adf"
	descriptionOutputFormatMarkdown = "markdown"
	descriptionOutputFormatText     = "text"
)

func parseDescriptionOutputFormat(format string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", descriptionOutputFormatText:
		return descriptionOutputFormatText, nil
	case descriptionOutputFormatMarkdown:
		return descriptionOutputFormatMarkdown, nil
	case descriptionOutputFormatADF:
		return descriptionOutputFormatADF, nil
	default:
		return "", apperr.NewValidationError(
			fmt.Sprintf("invalid --description-output-format %q", format),
			nil,
			apperr.WithDetails("valid formats: text, markdown, adf"),
		)
	}
}

func convertDescriptionOutputFields(value any, format string) any {
	if format == descriptionOutputFormatADF {
		return value
	}

	switch v := value.(type) {
	case map[string]any:
		converted := make(map[string]any, len(v))
		for key, nested := range v {
			if key == "description" {
				converted[key] = convertDescriptionOutputValue(nested, format)
				continue
			}
			converted[key] = convertDescriptionOutputFields(nested, format)
		}
		return converted
	case []any:
		converted := make([]any, 0, len(v))
		for _, item := range v {
			converted = append(converted, convertDescriptionOutputFields(item, format))
		}
		return converted
	default:
		return value
	}
}

func convertDescriptionOutputValue(value any, format string) any {
	if format == descriptionOutputFormatADF {
		return value
	}

	doc, ok := value.(map[string]any)
	if !ok || !isADFDocument(doc) {
		return convertDescriptionOutputFields(value, format)
	}

	if format == descriptionOutputFormatMarkdown {
		return adfDocumentToMarkdown(doc)
	}
	return adfDocumentToText(doc)
}

func isADFDocument(value map[string]any) bool {
	typeName, ok := value["type"].(string)
	return ok && typeName == "doc"
}

func adfDocumentToText(doc map[string]any) string {
	return strings.TrimSpace(strings.Join(adfBlockTextLines(doc, false), "\n"))
}

func adfDocumentToMarkdown(doc map[string]any) string {
	return strings.TrimSpace(strings.Join(adfBlockTextLines(doc, true), "\n"))
}

func adfBlockTextLines(node any, markdown bool) []string {
	nodeMap, ok := node.(map[string]any)
	if !ok {
		return nil
	}

	typeName, _ := nodeMap["type"].(string)
	switch typeName {
	case "doc":
		return adfChildBlockLines(nodeMap, markdown)
	case "paragraph":
		return []string{adfInlineText(nodeMap, markdown)}
	case "heading":
		text := adfInlineText(nodeMap, markdown)
		if !markdown {
			return []string{text}
		}
		level := adfHeadingLevel(nodeMap)
		return []string{strings.Repeat("#", level) + " " + text}
	case "bulletList":
		return adfListLines(nodeMap, markdown, false)
	case "orderedList":
		return adfListLines(nodeMap, markdown, true)
	case "listItem":
		return adfChildBlockLines(nodeMap, markdown)
	case "blockquote":
		lines := adfChildBlockLines(nodeMap, markdown)
		if !markdown {
			return lines
		}
		for i, line := range lines {
			lines[i] = "> " + line
		}
		return lines
	case "codeBlock":
		text := adfInlineText(nodeMap, false)
		if !markdown {
			return []string{text}
		}
		return []string{"```text\n" + text + "\n```"}
	default:
		text := adfInlineText(nodeMap, markdown)
		if text != "" {
			return []string{text}
		}
		return adfChildBlockLines(nodeMap, markdown)
	}
}

func adfChildBlockLines(node map[string]any, markdown bool) []string {
	children, ok := node["content"].([]any)
	if !ok {
		return nil
	}
	lines := make([]string, 0, len(children))
	for _, child := range children {
		lines = append(lines, adfBlockTextLines(child, markdown)...)
	}
	return lines
}

func adfListLines(node map[string]any, markdown, ordered bool) []string {
	children, ok := node["content"].([]any)
	if !ok {
		return nil
	}
	lines := make([]string, 0, len(children))
	for index, child := range children {
		itemLines := adfBlockTextLines(child, markdown)
		for itemIndex, line := range itemLines {
			if !markdown {
				lines = append(lines, line)
				continue
			}
			prefix := "  "
			if itemIndex == 0 {
				if ordered {
					prefix = fmt.Sprintf("%d. ", index+1)
				} else {
					prefix = "- "
				}
			}
			lines = append(lines, prefix+line)
		}
	}
	return lines
}

func adfHeadingLevel(node map[string]any) int {
	attrs, ok := node["attrs"].(map[string]any)
	if !ok {
		return 1
	}
	level, ok := attrs["level"].(float64)
	if !ok || level < 1 || level > 6 {
		return 1
	}
	return int(level)
}

func adfInlineText(node map[string]any, markdown bool) string {
	if typeName, _ := node["type"].(string); typeName == "text" {
		text, _ := node["text"].(string)
		if markdown {
			return applyADFMarkdownMarks(text, node)
		}
		return text
	}
	if typeName, _ := node["type"].(string); typeName == "hardBreak" {
		return "\n"
	}

	children, ok := node["content"].([]any)
	if !ok {
		return ""
	}
	var builder strings.Builder
	for _, child := range children {
		childMap, ok := child.(map[string]any)
		if !ok {
			continue
		}
		builder.WriteString(adfInlineText(childMap, markdown))
	}
	return builder.String()
}

func applyADFMarkdownMarks(text string, node map[string]any) string {
	marks, ok := node["marks"].([]any)
	if !ok {
		return text
	}
	for _, mark := range marks {
		markMap, ok := mark.(map[string]any)
		if !ok {
			continue
		}
		switch markMap["type"] {
		case "strong":
			text = "**" + text + "**"
		case "em":
			text = "*" + text + "*"
		case "code":
			text = "`" + text + "`"
		case "link":
			if href, ok := adfMarkHref(markMap); ok {
				text = "[" + text + "](" + href + ")"
			}
		}
	}
	return text
}

func adfMarkHref(mark map[string]any) (string, bool) {
	attrs, ok := mark["attrs"].(map[string]any)
	if !ok {
		return "", false
	}
	href, ok := attrs["href"].(string)
	return href, ok && href != ""
}
