package glob

import (
	"strings"
)

func Walk(pattern, root string) []string {
	if !hasSpecial(pattern) {
		return []string{pattern}
	}
	return []string{pattern}
}

func hasSpecial(str string) bool {
	return strings.IndexAny(str, "[*?") >= 0
}
