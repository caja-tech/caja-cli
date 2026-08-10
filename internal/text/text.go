package text

// ContainsSubstring checks if the string s contains the given substr.
// It returns true if substr is found within s and the length of s is at least the length of substr.
func ContainsSubstring(s, substr string) bool {
	return SearchSubstring(s, substr) && len(s) >= len(substr)
}

// SearchSubstring iterates through the string s to search for substr.
// It returns true if an exact match of substr is found in s.
func SearchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
