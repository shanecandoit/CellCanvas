package main

import (
	"fmt"
	"strconv"
	"strings"
)

// ColToLetters converts a zero-based column number to spreadsheet-style letters.
// Examples: 0 -> "A", 25 -> "Z", 26 -> "AA".
func ColToLetters(n int) string {
	if n < 0 {
		return "?"
	}
	s := ""
	for n >= 0 {
		rem := n % 26
		s = string('A'+rem) + s
		n = n/26 - 1
	}
	return s
}

// LettersToCol converts spreadsheet-style letters to a zero-based column number.
// Examples: "A" -> 0, "Z" -> 25, "AA" -> 26.
func LettersToCol(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return -1, fmt.Errorf("empty column string")
	}
	s = strings.ToUpper(s)
	res := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 'A' || c > 'Z' {
			return -1, fmt.Errorf("invalid column char: %c", c)
		}
		res = res*26 + int(c-'A') + 1
	}
	return res - 1, nil
}

// CellRef returns a string like "A1" for the given zero-based col and row.
func CellRef(col, row int) string {
	return fmt.Sprintf("%s%d", ColToLetters(col), row+1)
}

// ParseCellRef parses a cell reference like "A1" or "BC23" into zero-based
// column and row indices.
func ParseCellRef(ref string) (int, int, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return -1, -1, fmt.Errorf("empty cell reference")
	}
	// split letters prefix and digits suffix
	i := 0
	for i < len(ref) {
		c := ref[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			i++
			continue
		}
		break
	}
	if i == 0 {
		return -1, -1, fmt.Errorf("missing column letters in ref: %s", ref)
	}
	letters := ref[:i]
	digits := strings.TrimSpace(ref[i:])
	if digits == "" {
		return -1, -1, fmt.Errorf("missing row digits in ref: %s", ref)
	}
	col, err := LettersToCol(letters)
	if err != nil {
		return -1, -1, err
	}
	rowInt, err := strconv.Atoi(digits)
	if err != nil {
		return -1, -1, err
	}
	return col, rowInt - 1, nil
}
