package garba

import (
	"strings"

	"github.com/pocketbase/pocketbase/tools/security"
)

// Pass codes are what volunteers type when a QR won't scan, e.g. "KP7X-4MQ":
// 6 random characters plus a Luhn mod-30 check character.
//
//   - Random, so codes can't be guessed from each other (30^6 ≈ 730 million; with a few thousand
//     passes a made-up code hits a real one about 1 in 150,000 times).
//   - The alphabet leaves out look-alikes (0 O 1 I L U), so codes are easy to read out loud.
//   - The check character catches every single-character typo and most swapped neighbours,
//     and rejects ~29 in 30 made-up codes before any lookup.
//
// Keep in sync with web/src/passcode.ts.
const (
	codeAlphabet = "23456789ABCDEFGHJKMNPQRSTVWXYZ"
	codeBodyLen  = 6
)

func codeIndex(c byte) int { return strings.IndexByte(codeAlphabet, c) }

// codeCheck returns the Luhn mod-N check character for body.
func codeCheck(body string) byte {
	n := len(codeAlphabet)
	factor, sum := 2, 0
	for i := len(body) - 1; i >= 0; i-- {
		addend := factor * codeIndex(body[i])
		if factor == 2 {
			factor = 1
		} else {
			factor = 2
		}
		sum += addend/n + addend%n
	}
	return codeAlphabet[(n-sum%n)%n]
}

func formatCode(raw string) string { return raw[:4] + "-" + raw[4:] }

// newPassCode returns a fresh random code like "KP7X-4MQ".
func newPassCode() string {
	body := security.RandomStringWithAlphabet(codeBodyLen, codeAlphabet)
	return formatCode(body + string(codeCheck(body)))
}

// normalizePassCode cleans up a typed code (any case, spaces or dashes).
// ok is false if it isn't 7 valid characters or the check character doesn't match (a typo).
func normalizePassCode(s string) (code string, ok bool) {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if r == '-' || r == ' ' {
			continue
		}
		b.WriteRune(r)
	}
	raw := b.String()
	if len(raw) != codeBodyLen+1 {
		return "", false
	}
	for i := 0; i < len(raw); i++ {
		if codeIndex(raw[i]) < 0 {
			return "", false
		}
	}
	if codeCheck(raw[:codeBodyLen]) != raw[codeBodyLen] {
		return "", false
	}
	return formatCode(raw), true
}

// looksLikePassCode is true for input that is meant as a typed code (vs. a QR payload).
func looksLikePassCode(s string) bool {
	s = strings.TrimSpace(s)
	return !strings.HasPrefix(s, qrPrefix) && len(s) >= 5 && len(s) <= 12 && !strings.ContainsAny(s, "./:")
}
