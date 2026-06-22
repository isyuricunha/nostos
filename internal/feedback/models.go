package feedback

import "time"

const (
	RatingPositive = "positive"
	RatingNegative = "negative"
)

var NegativeReasons = map[string]struct{}{
	"Incorrect information":       {},
	"Too long":                    {},
	"Too technical":               {},
	"Did not follow instructions": {},
	"Inappropriate tone":          {},
	"Invented information":        {},
	"Ignored memories":            {},
	"Other":                       {},
}

type MessageFeedback struct {
	ID         string    `json:"id"`
	MessageID  string    `json:"message_id"`
	UserID     string    `json:"user_id"`
	Rating     string    `json:"rating"`
	Reason     string    `json:"reason,omitempty"`
	Comment    string    `json:"comment,omitempty"`
	ProviderID string    `json:"provider_id,omitempty"`
	Model      string    `json:"model,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type FeedbackInput struct {
	Rating  string `json:"rating"`
	Reason  string `json:"reason,omitempty"`
	Comment string `json:"comment,omitempty"`
}

type FeedbackStats struct {
	Positive int `json:"positive"`
	Negative int `json:"negative"`
}

type PrincipalContext struct {
	WorkspaceID string
	UserID      string
}
