package output

import (
	"fmt"
	"html"
	"strings"

	"mastodoncli/internal/mastodon"
)

func PrintStatuses(statuses []mastodon.Status) {
	if len(statuses) == 0 {
		fmt.Println("No statuses returned.")
		return
	}

	for _, item := range statuses {
		display := &item
		boostedBy := ""
		if item.Reblog != nil {
			boostedBy = fmt.Sprintf("@%s", item.Account.Acct)
			display = item.Reblog
		}

		name := strings.TrimSpace(StripHTML(display.Account.DisplayName))
		body := WrapText(StripHTML(display.Content), 80)

		fmt.Println("----")
		if name != "" && name != display.Account.Acct {
			fmt.Printf("%sAuthor:%s %s (@%s)\n", colorCyan, colorReset, name, display.Account.Acct)
		} else {
			fmt.Printf("%sAuthor:%s @%s\n", colorCyan, colorReset, display.Account.Acct)
		}
		fmt.Printf("%sTime:%s   %s\n", colorYellow, colorReset, display.CreatedAt)
		if boostedBy != "" {
			fmt.Printf("Boost:  %s\n", boostedBy)
		}
		fmt.Println("Text:")
		fmt.Println(body)
		fmt.Println()
	}
}

func StripHTML(input string) string {
	var builder strings.Builder
	builder.Grow(len(input))

	inTag := false
	for _, r := range input {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				builder.WriteRune(r)
			}
		}
	}

	return strings.TrimSpace(html.UnescapeString(builder.String()))
}

func WrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}

	var builder strings.Builder
	lineLen := 0
	for _, word := range words {
		if lineLen == 0 {
			builder.WriteString(word)
			lineLen = len(word)
			continue
		}

		if lineLen+1+len(word) > width {
			builder.WriteByte('\n')
			builder.WriteString(word)
			lineLen = len(word)
			continue
		}

		builder.WriteByte(' ')
		builder.WriteString(word)
		lineLen += 1 + len(word)
	}

	return builder.String()
}

const (
	colorReset  = "\033[0m"
	colorCyan   = "\033[36m"
	colorYellow = "\033[33m"
)
