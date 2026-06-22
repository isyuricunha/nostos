package feedback

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var ErrInvalidInput = errors.New("invalid feedback input")

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListForConversation(ctx context.Context, principal PrincipalContext, conversationID string) ([]MessageFeedback, error) {
	if strings.TrimSpace(conversationID) == "" {
		return nil, fmt.Errorf("%w: conversation_id is required", ErrInvalidInput)
	}
	return s.repo.ListForConversation(ctx, principal.WorkspaceID, principal.UserID, conversationID)
}

func (s *Service) Upsert(ctx context.Context, principal PrincipalContext, messageID string, input FeedbackInput) (MessageFeedback, error) {
	normalized, err := normalizeInput(input)
	if err != nil {
		return MessageFeedback{}, err
	}
	return s.repo.Upsert(ctx, principal.WorkspaceID, principal.UserID, messageID, normalized)
}

func (s *Service) Delete(ctx context.Context, principal PrincipalContext, messageID string) error {
	if strings.TrimSpace(messageID) == "" {
		return fmt.Errorf("%w: message_id is required", ErrInvalidInput)
	}
	return s.repo.Delete(ctx, principal.WorkspaceID, principal.UserID, messageID)
}

func (s *Service) Stats(ctx context.Context, principal PrincipalContext) (FeedbackStats, error) {
	return s.repo.Stats(ctx, principal.WorkspaceID)
}

func normalizeInput(input FeedbackInput) (FeedbackInput, error) {
	rating := strings.TrimSpace(input.Rating)
	if rating != RatingPositive && rating != RatingNegative {
		return FeedbackInput{}, fmt.Errorf("%w: rating must be positive or negative", ErrInvalidInput)
	}
	reason := strings.TrimSpace(input.Reason)
	if rating == RatingNegative {
		if _, ok := NegativeReasons[reason]; !ok {
			return FeedbackInput{}, fmt.Errorf("%w: negative feedback reason is invalid", ErrInvalidInput)
		}
	} else {
		reason = ""
	}
	return FeedbackInput{
		Rating:  rating,
		Reason:  reason,
		Comment: strings.TrimSpace(input.Comment),
	}, nil
}
