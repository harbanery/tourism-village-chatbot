package chat

import (
	"github.com/gofiber/fiber/v2"
)

type ChatHandler interface {
	CreateSession(ctx *fiber.Ctx) error
	SendMessage(ctx *fiber.Ctx) error
	GetHistory(ctx *fiber.Ctx) error
}

type chatHandler struct {
	UseCase ChatUseCase
}

func NewChatHandler(useCase ChatUseCase) ChatHandler {
	return &chatHandler{
		UseCase: useCase,
	}
}

func (h *chatHandler) CreateSession(ctx *fiber.Ctx) error {
	res := h.UseCase.CreateSession(ctx)
	return ctx.Status(statusFromCode(res.Code)).JSON(res)
}

func (h *chatHandler) SendMessage(ctx *fiber.Ctx) error {
	res := h.UseCase.SendMessage(ctx)
	return ctx.Status(statusFromCode(res.Code)).JSON(res)
}

func (h *chatHandler) GetHistory(ctx *fiber.Ctx) error {
	res := h.UseCase.GetHistory(ctx)
	return ctx.Status(statusFromCode(res.Code)).JSON(res)
}

func statusFromCode(code int) int {
	if code == 0 {
		return fiber.StatusOK
	}
	return code
}
