package utils

func InStrings(a []string, s string) bool {
	for _, v := range a {
		if v == s {
			return true
		}
	}
	return false
}

func IsSpace(b byte) bool {
	return b == ' ' || b == '\t'
}
