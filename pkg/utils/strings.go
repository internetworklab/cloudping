package utils

import "strings"

// Takes an ISO 3166-1 alpha-2 code, returns a flag unicode string
func Alpha2CountryCodeToUnicode(alpha2 string) string {
	if len(alpha2) != 2 {
		return ""
	}

	// Convert to uppercase
	alpha2 = strings.ToUpper(alpha2)

	// Get the two letters
	letter1 := rune(alpha2[0])
	letter2 := rune(alpha2[1])

	// Validate they are A-Z
	if letter1 < 'A' || letter1 > 'Z' || letter2 < 'A' || letter2 > 'Z' {
		return ""
	}

	// Calculate regional indicator symbols
	// Regional indicator A is at U+1F1E6, B is at U+1F1E7, etc.
	flag1 := '\U0001F1E6' + (letter1 - 'A')
	flag2 := '\U0001F1E6' + (letter2 - 'A')

	// Combine them to form the flag emoji
	return string(flag1) + string(flag2)
}

func SplitBySpace(s string) []string {
	cliSegsRaw := strings.Split(s, " ")
	cliSegs := make([]string, 0)
	for _, x := range cliSegsRaw {
		x = strings.TrimSpace(x)
		if len(x) > 0 {
			cliSegs = append(cliSegs, x)
		}
	}
	return cliSegs
}
