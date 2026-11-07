package knowledge

import (
	"github.com/gofiber/fiber/v2"
)

type KnowledgeHandler interface {
	IngestText(ctx *fiber.Ctx) error
	UploadDocument(ctx *fiber.Ctx) error
	ListDocuments(ctx *fiber.Ctx) error
	DeleteDocument(ctx *fiber.Ctx) error
	Search(ctx *fiber.Ctx) error
}

type knowledgeHandler struct {
	UseCase KnowledgeUseCase
}

func NewKnowledgeHandler(useCase KnowledgeUseCase) KnowledgeHandler {
	return &knowledgeHandler{
		UseCase: useCase,
	}
}

func (h *knowledgeHandler) IngestText(ctx *fiber.Ctx) error {
	res := h.UseCase.IngestText(ctx)
	return ctx.Status(statusFromCode(res.Code)).JSON(res)
}

func (h *knowledgeHandler) UploadDocument(ctx *fiber.Ctx) error {
	res := h.UseCase.UploadDocument(ctx)
	return ctx.Status(statusFromCode(res.Code)).JSON(res)
}

func (h *knowledgeHandler) ListDocuments(ctx *fiber.Ctx) error {
	res := h.UseCase.ListDocuments(ctx)
	return ctx.Status(statusFromCode(res.Code)).JSON(res)
}

func (h *knowledgeHandler) DeleteDocument(ctx *fiber.Ctx) error {
	res := h.UseCase.DeleteDocument(ctx)
	return ctx.Status(statusFromCode(res.Code)).JSON(res)
}

func (h *knowledgeHandler) Search(ctx *fiber.Ctx) error {
	res := h.UseCase.Search(ctx)
	return ctx.Status(statusFromCode(res.Code)).JSON(res)
}

func statusFromCode(code int) int {
	if code == 0 {
		return fiber.StatusOK
	}
	return code
}
