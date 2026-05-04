package http

// errCode extracts error.code from a decoded envelope map. Returns "" when
// the envelope has no error object or the code is missing.
func errCode(env map[string]any) string {
	if env == nil {
		return ""
	}
	errObj, _ := env["error"].(map[string]any)
	if errObj == nil {
		return ""
	}
	code, _ := errObj["code"].(string)
	return code
}
