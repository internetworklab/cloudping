package utils

func CheckSortedStringsDup(s []string) bool {
	for i := 1; i < len(s); i++ {
		if s[i-1] == s[i] {
			return true
		}
	}
	return false
}

func Dedup(s []string) []string {
	tmpMap := make(map[string]bool)
	for _, x := range s {
		tmpMap[x] = true
	}
	result := make([]string, 0)
	for x := range tmpMap {
		result = append(result, x)
	}
	return result
}
