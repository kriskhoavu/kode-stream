package scanner

import (
	"strconv"
	"strings"
	"unicode"
)

func naturalLess(left, right string) bool {
	leftParts := naturalParts(left)
	rightParts := naturalParts(right)
	for i := 0; i < len(leftParts) && i < len(rightParts); i++ {
		a, b := leftParts[i], rightParts[i]
		if a.number && b.number {
			if a.numberValue != b.numberValue {
				return a.numberValue < b.numberValue
			}
			continue
		}
		if a.value != b.value {
			return a.value < b.value
		}
	}
	return len(leftParts) < len(rightParts)
}

type naturalPart struct {
	value       string
	number      bool
	numberValue int
}

func naturalParts(input string) []naturalPart {
	var parts []naturalPart
	for i := 0; i < len(input); {
		start := i
		isNumber := unicode.IsDigit(rune(input[i]))
		for i < len(input) && unicode.IsDigit(rune(input[i])) == isNumber {
			i++
		}
		value := strings.ToLower(input[start:i])
		part := naturalPart{value: value, number: isNumber}
		if isNumber {
			part.numberValue, _ = strconv.Atoi(value)
		}
		parts = append(parts, part)
	}
	return parts
}
