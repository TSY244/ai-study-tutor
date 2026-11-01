package claudegateway

import aipkg "github.com/TSY244/ai-study-tutor/backend/internal/ai"

const (
	ProtocolAnthropic = "anthropic"
	ProtocolOpenAI    = "openai"
)

// Empty protocol is the legacy on-disk representation of Anthropic.
func EffectiveProtocol(protocol string) string {
	if protocol == "" {
		return ProtocolAnthropic
	}
	return protocol
}

func validProtocol(protocol string) bool {
	return EffectiveProtocol(protocol) == ProtocolAnthropic || protocol == ProtocolOpenAI
}

// OpenAI never inherits the deployment's Anthropic endpoint.
func EffectiveKeyBaseURL(key Key, configured string) string {
	if EffectiveProtocol(key.Protocol) == ProtocolOpenAI {
		return aipkg.OpenAIBaseURL(key.BaseURL)
	}
	return EffectiveBaseURL(key.BaseURL, configured)
}

func keyEndpoint(key Key, configured string) string {
	base := EffectiveKeyBaseURL(key, configured)
	if EffectiveProtocol(key.Protocol) == ProtocolOpenAI {
		return aipkg.OpenAIChatEndpoint(base)
	}
	return messagesEndpoint(base)
}
