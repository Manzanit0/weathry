package tgram

import "strings"

func ExtractCommandQuery(text string) string {
	strs := strings.Split(text, " ")
	if len(strs) == 1 {
		return ""
	}

	return strings.Join(strs[1:], " ")
}
