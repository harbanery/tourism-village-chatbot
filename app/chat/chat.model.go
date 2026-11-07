package chat

import (
	"time"

	"support-agent-golang/entity"
)

type CreateSessionRequest struct {
	UserID string `json:"user_id"`
	Title  string `json:"title"`
}

type SessionResponse struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SendMessageRequest struct {
	SessionID string `json:"session_id" validate:"required,uuid4"`
	Message   string `json:"message" validate:"required"`
}

type SendMessageData struct {
	SessionID string                 `json:"session_id"`
	Answer    string                 `json:"answer"`
	Sources   []entity.MessageSource `json:"sources"`
	LatencyMs int64                  `json:"latency_ms"`
}

type MessageResponse struct {
	ID        string                 `json:"id"`
	SessionID string                 `json:"session_id"`
	Role      string                 `json:"role"`
	Content   string                 `json:"content"`
	Sources   []entity.MessageSource `json:"sources"`
	LatencyMs int64                  `json:"latency_ms"`
	CreatedAt time.Time              `json:"created_at"`
}

func ToSessionResponse(session entity.ChatSession) SessionResponse {
	return SessionResponse{
		ID:        session.ID,
		UserID:    session.UserID,
		Title:     session.Title,
		CreatedAt: session.CreatedAt,
		UpdatedAt: session.UpdatedAt,
	}
}

func ToMessageResponse(message entity.ChatMessage) MessageResponse {
	return MessageResponse{
		ID:        message.ID,
		SessionID: message.SessionID,
		Role:      message.Role,
		Content:   message.Content,
		Sources:   message.Sources,
		LatencyMs: message.LatencyMs,
		CreatedAt: message.CreatedAt,
	}
}
