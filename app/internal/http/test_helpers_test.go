package http

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
