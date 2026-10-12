// Package redact takes secrets out of text R3V keeps (an agent's prompts,
// for steps): R3V's connection codes, the shapes of common keys and
// tokens, private keys, and long random-looking strings, each replaced by
// [redacted].
package redact

import (
	"math"
	"regexp"
	"strings"
)

// Mark replaces what was taken out.
const Mark = "[redacted]"

var patterns = []*regexp.Regexp{
	regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?(-----END [A-Z ]*PRIVATE KEY-----|$)`),
	regexp.MustCompile(`r3v-[a-z0-9]*:[A-Za-z0-9_\-=]+`),                         // connection codes
	regexp.MustCompile(`\b(AKIA|ASIA)[0-9A-Z]{16}\b`),                            // AWS access keys
	regexp.MustCompile(`\bsk-[A-Za-z0-9_\-]{20,}`),                               // API keys (sk-…, sk-ant-…)
	regexp.MustCompile(`\b(ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{30,}\b`),             // GitHub tokens
	regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{30,}\b`),                       // GitHub fine-grained tokens
	regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9\-]{10,}`),                        // Slack tokens
	regexp.MustCompile(`\bAIza[0-9A-Za-z_\-]{35}\b`),                             // Google API keys
	regexp.MustCompile(`\beyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+`), // JWTs
	regexp.MustCompile(`(?i)\b(password|passwd|secret|token|api[_-]?key)\s*[:=]\s*\S+`),
}

// word is a run that may be a key: letters, digits and the characters keys
// are written with.
var word = regexp.MustCompile(`[A-Za-z0-9+/=_\-]{32,}`)

// Text is text with the secrets it seems to hold replaced by Mark.
func Text(s string) string {
	for _, p := range patterns {
		s = p.ReplaceAllString(s, Mark)
	}
	return word.ReplaceAllStringFunc(s, func(w string) string {
		if randomLooking(w) {
			return Mark
		}
		return w
	})
}

// randomLooking: upper and lower case and digits mixed, spread like random
// text (a hex hash or a long word isn't).
func randomLooking(w string) bool {
	var upper, lower, digit bool
	for _, c := range w {
		switch {
		case c >= 'A' && c <= 'Z':
			upper = true
		case c >= 'a' && c <= 'z':
			lower = true
		case c >= '0' && c <= '9':
			digit = true
		}
	}
	if !(upper && lower && digit) || strings.Contains(w, "--") {
		return false
	}
	return entropy(w) > 4.0
}

// entropy is the bits per character of w.
func entropy(w string) float64 {
	counts := map[rune]float64{}
	for _, c := range w {
		counts[c]++
	}
	n := float64(len(w))
	var bits float64
	for _, c := range counts {
		p := c / n
		bits -= p * math.Log2(p)
	}
	return bits
}
