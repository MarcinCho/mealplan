package domain

// HasControl reports whether s contains a NUL, another C0 control character
// (other than tab, newline and carriage return) or DEL. Postgres cannot store
// NUL in text, and the rest have no place in user-visible text.
func HasControl(s string) bool {
	for _, r := range s {
		if r == '\t' || r == '\n' || r == '\r' {
			continue
		}
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}
