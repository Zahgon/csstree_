package jsutil

import (
	"math"
	"strconv"
	"strings"
)

// NumberToString is String(number) in JavaScript: the shortest decimal that reads back as the same
// double, written without an exponent while the decimal exponent is between -7 and 21.
func NumberToString(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	case f == 0:
		return "0"
	}
	if f < 0 {
		return "-" + NumberToString(-f)
	}
	// The shortest round-tripping decimal digits and the decimal exponent of the first digit.
	shortest := strconv.FormatFloat(f, 'e', -1, 64)
	mantissa, exponent := splitExponential(shortest)
	digits := strings.Replace(mantissa, ".", "", 1)
	n := exponent + 1 // the number is 0.<digits> * 10^n
	k := len(digits)
	switch {
	case k <= n && n <= 21:
		return digits + strings.Repeat("0", n-k)
	case 0 < n && n <= 21:
		return digits[:n] + "." + digits[n:]
	case -6 < n && n <= 0:
		return "0." + strings.Repeat("0", -n) + digits
	default:
		e := n - 1
		sign := "+"
		if e < 0 {
			sign = "-"
			e = -e
		}
		head := digits[:1]
		if k > 1 {
			head += "." + digits[1:]
		}
		return head + "e" + sign + strconv.Itoa(e)
	}
}

func splitExponential(s string) (string, int) {
	i := strings.IndexByte(s, 'e')
	exponent, _ := strconv.Atoi(s[i+1:])
	return s[:i], exponent
}

// ParseFloat is the JavaScript parseFloat: it reads the longest decimal prefix of the string and
// is NaN when there is none.
func ParseFloat(s Str) float64 {
	text := s.String()
	i := 0
	for i < len(text) && isJSWhiteSpaceByte(text[i]) {
		i++
	}
	start := i
	if i < len(text) && (text[i] == '+' || text[i] == '-') {
		i++
	}
	if strings.HasPrefix(text[i:], "Infinity") {
		if text[start] == '-' {
			return math.Inf(-1)
		}
		return math.Inf(1)
	}
	digits := i
	for i < len(text) && text[i] >= '0' && text[i] <= '9' {
		i++
	}
	if i < len(text) && text[i] == '.' {
		i++
		for i < len(text) && text[i] >= '0' && text[i] <= '9' {
			i++
		}
	}
	if i == digits || (i == digits+1 && text[digits] == '.') {
		return math.NaN()
	}
	mantissaEnd := i
	if i < len(text) && (text[i] == 'e' || text[i] == 'E') {
		j := i + 1
		if j < len(text) && (text[j] == '+' || text[j] == '-') {
			j++
		}
		k := j
		for k < len(text) && text[k] >= '0' && text[k] <= '9' {
			k++
		}
		if k > j {
			mantissaEnd = k
		}
	}
	value, err := strconv.ParseFloat(text[start:mantissaEnd], 64)
	if err != nil {
		if strings.Contains(err.Error(), "value out of range") {
			return value
		}
		return math.NaN()
	}
	return value
}

func isJSWhiteSpaceByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\v' || c == '\f'
}

// ParseIntRadix is the JavaScript parseInt(string, radix) for the radixes the original uses.
func ParseIntRadix(s Str, radix int) float64 {
	text := strings.TrimLeft(s.String(), " \t\n\r\v\f")
	i := 0
	negative := false
	if i < len(text) && (text[i] == '+' || text[i] == '-') {
		negative = text[i] == '-'
		i++
	}
	start := i
	value := 0.0
	for i < len(text) {
		d := digitValue(text[i])
		if d < 0 || d >= radix {
			break
		}
		value = value*float64(radix) + float64(d)
		i++
	}
	if i == start {
		return math.NaN()
	}
	if negative {
		return -value
	}
	return value
}

func digitValue(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'z':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'Z':
		return int(c-'A') + 10
	default:
		return -1
	}
}
