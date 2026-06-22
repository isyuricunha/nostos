package replies

import "time"

type Preset struct {
	ID                string    `json:"id"`
	WorkspaceID       string    `json:"workspace_id"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	PromptInstruction string    `json:"prompt_instruction"`
	Icon              string    `json:"icon"`
	SortOrder         int       `json:"sort_order"`
	Active            bool      `json:"active"`
	SystemDefault     bool      `json:"system_default"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type Draft struct {
	ID                string    `json:"id"`
	WorkspaceID       string    `json:"workspace_id"`
	SourceMessageID   string    `json:"source_message_id"`
	PresetID          string    `json:"preset_id,omitempty"`
	PresetName        string    `json:"preset_name"`
	CustomInstruction string    `json:"custom_instruction"`
	GeneratedDraft    string    `json:"generated_draft"`
	ProviderID        string    `json:"provider_id,omitempty"`
	Model             string    `json:"model,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type PresetInput struct {
	Name              string `json:"name"`
	Description       string `json:"description"`
	PromptInstruction string `json:"prompt_instruction"`
	Icon              string `json:"icon"`
	SortOrder         int    `json:"sort_order"`
	Active            bool   `json:"active"`
}

type DraftInput struct {
	SourceMessageID   string `json:"source_message_id"`
	PresetID          string `json:"preset_id"`
	CustomInstruction string `json:"custom_instruction,omitempty"`
	ProviderID        string `json:"provider_id,omitempty"`
	Model             string `json:"model,omitempty"`
}

type SourceMessage struct {
	ID         string
	Content    string
	ProviderID string
	Model      string
}

type PrincipalContext struct {
	WorkspaceID string
	UserID      string
}
