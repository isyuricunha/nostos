package providers

import (
	"strings"
)

var validCapabilities = map[string]struct{}{
	"chat":             {},
	"vision":           {},
	"tools":            {},
	"reasoning":        {},
	"embedding":        {},
	"audio":            {},
	"image_generation": {},
}

func friendlyModelName(modelID string) string {
	trimmed := strings.TrimSpace(modelID)
	if trimmed == "" {
		return ""
	}
	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == '/' || r == ':' || r == '|'
	})
	return strings.TrimSpace(parts[len(parts)-1])
}

func inferCapabilities(modelID string) []string {
	value := strings.ToLower(modelID)
	capabilities := []string{}
	switch {
	case strings.Contains(value, "embed"):
		capabilities = append(capabilities, "embedding")
	case strings.Contains(value, "audio") || strings.Contains(value, "whisper") || strings.Contains(value, "tts"):
		capabilities = append(capabilities, "audio")
	case strings.Contains(value, "image") && !strings.Contains(value, "vision"):
		capabilities = append(capabilities, "image_generation")
	default:
		capabilities = append(capabilities, "chat")
	}
	if strings.Contains(value, "vision") || strings.Contains(value, "vl") || strings.Contains(value, "gpt-4o") {
		capabilities = appendCapability(capabilities, "vision")
	}
	if strings.Contains(value, "tool") || strings.Contains(value, "gpt") || strings.Contains(value, "claude") || strings.Contains(value, "qwen") {
		capabilities = appendCapability(capabilities, "tools")
	}
	if strings.Contains(value, "reason") || strings.Contains(value, "thinking") || strings.Contains(value, "r1") {
		capabilities = appendCapability(capabilities, "reasoning")
	}
	return capabilities
}

func sanitizeCapabilities(values []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if _, ok := validCapabilities[value]; !ok {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func appendCapability(values []string, capability string) []string {
	for _, value := range values {
		if value == capability {
			return values
		}
	}
	return append(values, capability)
}

func normalizedModelSearch(parts ...string) string {
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))
		if part != "" {
			values = append(values, part)
		}
	}
	return strings.Join(values, " ")
}

func coalesceString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func validModelRole(role string) bool {
	switch role {
	case ModelRoleChat, ModelRoleUtility, ModelRoleVision:
		return true
	default:
		return false
	}
}
