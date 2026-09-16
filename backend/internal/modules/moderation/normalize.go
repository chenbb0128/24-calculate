package moderation

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const maxModerationTextRunes = 256

func NormalizeText(value string) (string, error) {
	value = norm.NFKC.String(value)
	var normalized strings.Builder
	for _, r := range value {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			continue
		}
		if unicode.IsSpace(r) {
			normalized.WriteByte(' ')
			continue
		}
		normalized.WriteRune(r)
	}
	value = strings.Join(strings.Fields(normalized.String()), " ")
	if value == "" {
		return "", fmt.Errorf("moderation text is empty")
	}
	if len([]rune(value)) > maxModerationTextRunes {
		return "", fmt.Errorf("moderation text is too long")
	}
	return value, nil
}
