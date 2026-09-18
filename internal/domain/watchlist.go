package domain

import (
	"strings"
)

const MaxWatchlist = 100

func NormalizeCode(input string) string {
	s := strings.ToUpper(strings.TrimSpace(input))
	s = strings.TrimSuffix(s, ".JK")
	return s
}

func YahooSymbol(code string) string {
	return NormalizeCode(code) + ".JK"
}
