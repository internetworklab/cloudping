package bot

import (
	"regexp"
	"strings"
)

func TrimCommandPrefix(updateText string) string {
	// Remove any /command or /command@someone prefix
	// Pattern matches: /command@bot or /command followed by optional whitespace
	re := regexp.MustCompile(`^/\w+(@\w+)?\s*`)
	text := strings.TrimSpace(updateText)
	rest := re.ReplaceAllString(text, "")
	return strings.TrimSpace(rest)
}
