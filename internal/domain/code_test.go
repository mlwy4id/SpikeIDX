package domain

import (
	"errors"
	"testing"
)

func TestParseCode(t *testing.T) {
	cases := []struct {
		in      string
		want    Code
		wantErr error
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
		got, err := ParseCode(tc.in)
		if tc.wantErr != nil {
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("ParseCode(%q): expected %v, got %v", tc.in, tc.wantErr, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseCode(%q): unexpected error %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseCode(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCodeYahooSymbol(t *testing.T) {
	if got := MustParseCode("bbca").YahooSymbol(); got != "BBCA.JK" {
		t.Fatalf("got %q", got)
	}
}
