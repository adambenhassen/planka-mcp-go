package server

import "regexp"

// pathParamPattern matches a {name} path parameter placeholder. It also matches
// the {name} inside the legacy ${name} custom-field variant, mirroring the TS
// regex /\{(\w+)\}/g.
var pathParamPattern = regexp.MustCompile(`\{(\w+)\}`)

// uriComponentUnreserved reports whether r is left unescaped by JavaScript's
// encodeURIComponent (A-Za-z0-9 and -_.!~*'()).
func uriComponentUnreserved(r byte) bool {
	switch {
	case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		return true
	}
	switch r {
	case '-', '_', '.', '!', '~', '*', '\'', '(', ')':
		return true
	}
	return false
}

// encodeURIComponent percent-encodes s like JavaScript's encodeURIComponent,
// used to escape substituted path-parameter values.
func encodeURIComponent(s string) string {
	const hex = "0123456789ABCDEF"
	var b []byte
	for i := range len(s) {
		c := s[i]
		if uriComponentUnreserved(c) {
			b = append(b, c)
			continue
		}
		b = append(b, '%', hex[c>>4], hex[c&0x0f])
	}
	return string(b)
}
