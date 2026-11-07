package route

import (
	"github.com/gofiber/fiber/v2"

	"support-agent-golang/app/chat"
	"support-agent-golang/app/knowledge"
)

type RouteConfig struct {
	App              *fiber.App
	APIKeyMiddleware fiber.Handler
	KnowledgeHandler knowledge.KnowledgeHandler
	ChatHandler      chat.ChatHandler
}

func (c *RouteConfig) Setup() {
	c.Knowledge()
	c.Chat()
}

func (c *RouteConfig) Knowledge() {
	knowledgeRoute := c.App.Group("/api/knowledge", c.APIKeyMiddleware)

	knowledgeRoute.Post("/text", c.KnowledgeHandler.IngestText)
	knowledgeRoute.Post("/documents", c.KnowledgeHandler.UploadDocument)
	knowledgeRoute.Get("/documents", c.KnowledgeHandler.ListDocuments)
	knowledgeRoute.Delete("/documents/:id", c.KnowledgeHandler.DeleteDocument)
	knowledgeRoute.Post("/search", c.KnowledgeHandler.Search)
}

func (c *RouteConfig) Chat() {
	chatRoute := c.App.Group("/api/chat", c.APIKeyMiddleware)

	chatRoute.Post("/sessions", c.ChatHandler.CreateSession)
	chatRoute.Post("/messages", c.ChatHandler.SendMessage)
	chatRoute.Get("/sessions/:id/messages", c.ChatHandler.GetHistory)
}
