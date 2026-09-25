package domain

import (
	"fmt"
	"regexp"
)

var codePattern = regexp.MustCompile(`^[A-Z]{3,4}$`)

type Code string

func ParseCode(input string) (Code, error) {
	s := NormalizeCode(input)
	if !codePattern.MatchString(s) {
		return "", fmt.Errorf("%w: %q", ErrInvalidCode, input)
	}
	return Code(s), nil
}

func MustParseCode(input string) Code {
	c, err := ParseCode(input)
	if err != nil {
		panic(err)
	}
	return c
}

func (c Code) String() string { return string(c) }

func (c Code) YahooSymbol() string { return string(c) + ".JK" }
