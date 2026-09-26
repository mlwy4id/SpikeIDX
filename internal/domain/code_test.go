package domain

import (
	"errors"
	"testing"
)

func TestParseCode(t *testing.T) {
	cases := []struct {
		input       string
		expected    Code
		expectedErr error
	}{
		{"BBCA", "BBCA", nil},
		{"bbca", "BBCA", nil},
		{"bbca.jk", "BBCA", nil},
		{"  tlkm  ", "TLKM", nil},
		{"GOTO", "GOTO", nil},
		{"", "", ErrInvalidCode},
		{"   ", "", ErrInvalidCode},
		{"BB", "", ErrInvalidCode},
		{"BBCA1", "", ErrInvalidCode},
		{"ABCDE", "", ErrInvalidCode},
		{"BB-CA", "", ErrInvalidCode},
		{"B CA", "", ErrInvalidCode},
		{"BBCA.JK.JK", "", ErrInvalidCode},
	}

	for _, tc := range cases {
		got, err := ParseCode(tc.input)

		if tc.expectedErr != nil {
			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("ParseCode(%q): expected %v, got %v", tc.input, tc.expectedErr, err)
			}
			continue
		}

		if err != nil {
			t.Errorf("ParseCode(%q): unexpected error %v", tc.input, err)
			continue
		}

		if got != tc.expected {
			t.Errorf("ParseCode(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestCodeYahooSymbol(t *testing.T) {
	if got := MustParseCode("bbca").YahooSymbol(); got != "BBCA.JK" {
		t.Fatalf("got %q", got)
	}
}
