package text

func ContainsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && SearchSubstring(s, substr)
}

func SearchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
