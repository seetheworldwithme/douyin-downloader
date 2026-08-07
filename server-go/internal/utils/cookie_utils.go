package utils

import "strings"

var invalidCookieNameChars = map[byte]bool{
	'(': true, ')': true, '<': true, '>': true, '@': true,
	',': true, ';': true, ':': true, '\\': true, '"': true,
	'/': true, '[': true, ']': true, '?': true, '=': true,
	'{': true, '}': true, ' ': true, '\t': true, '\r': true, '\n': true,
}

// IsValidCookieName checks if a string is a valid RFC6265 cookie name.
func IsValidCookieName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		ch := name[i]
		if ch < 33 || ch > 126 {
			return false
		}
		if invalidCookieNameChars[ch] {
			return false
		}
	}
	return true
}

// SanitizeCookies filters and trims a cookie map.
func SanitizeCookies(cookies map[string]string) map[string]string {
	sanitized := make(map[string]string)
	for k, v := range cookies {
		if k == "" {
			continue
		}
		key := strings.TrimSpace(k)
		if !IsValidCookieName(key) {
			continue
		}
		sanitized[key] = strings.TrimSpace(v)
	}
	return sanitized
}

// ParseCookieHeader parses a "key=val; key2=val2" string into a map.
func ParseCookieHeader(header string) map[string]string {
	parsed := make(map[string]string)
	if header == "" {
		return parsed
	}
	for _, item := range strings.Split(header, ";") {
		item = strings.TrimSpace(item)
		if item == "" || !strings.Contains(item, "=") {
			continue
		}
		idx := strings.IndexByte(item, '=')
		key := strings.TrimSpace(item[:idx])
		val := strings.TrimSpace(item[idx+1:])
		if !IsValidCookieName(key) {
			continue
		}
		parsed[key] = val
	}
	return parsed
}
