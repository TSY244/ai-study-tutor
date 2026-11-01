package piruntime

const DefaultScript = "/app/pi-runtime/agent.mjs"

// Args contains no credentials. All jobs use the same packaged SDK bridge.
func Args(script, tools, prompt string) []string {
	if script == "" {
		script = DefaultScript
	}
	return []string{script, "--tools", tools, "--prompt", prompt}
}
