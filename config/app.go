package config

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	"tourism-village-chatbot/app/chat"
	"tourism-village-chatbot/app/knowledge"
	"tourism-village-chatbot/entity"
	"tourism-village-chatbot/helper"
	"tourism-village-chatbot/middleware"
	"tourism-village-chatbot/route"
)

type BootstrapConfig struct {
	DB       *gorm.DB
	App      *fiber.App
	Log      *logrus.Logger
	Validate *validator.Validate
	Config   *viper.Viper
}

type Version struct {
	VersionNumber string `json:"version_number"`
	Service       string `json:"service"`
}

func Bootstrap(config *BootstrapConfig) {
	config.App.Get("/version/development", func(c *fiber.Ctx) error {
		res := Version{
			VersionNumber: "1.0.0",
			Service:       "support agent service",
		}

		return c.JSON(res)
	})

	config.App.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	// DATABASE MIGRATION
	err := config.DB.AutoMigrate(
		&entity.Document{},
		&entity.DocumentChunk{},
		&entity.ChatSession{},
		&entity.ChatMessage{},
	)
	if err != nil {
		config.Log.Fatalf("failed to migrate database: %v", err)
	}

	// VERTEX AI CLIENT
	vertexClient := helper.NewVertexAIClient(config.Config, config.Log)

	// KNOWLEDGE MODULE
	knowledgeRepository := knowledge.NewKnowledgeRepository(config.Log)
	knowledgeUseCase := knowledge.NewKnowledgeUseCase(config.DB, config.Log, config.Validate, knowledgeRepository, config.Config, vertexClient)
	knowledgeHandler := knowledge.NewKnowledgeHandler(knowledgeUseCase)

	// CHAT MODULE
	chatRepository := chat.NewChatRepository(config.Log)
	chatUseCase := chat.NewChatUseCase(config.DB, config.Log, config.Validate, chatRepository, config.Config, vertexClient)
	chatHandler := chat.NewChatHandler(chatUseCase)

	// MIDDLEWARE
	apiKeyMiddleware := middleware.NewAPIKeyMiddleware(config.Log, config.Config)

	routeConfig := route.RouteConfig{
		App:              config.App,
		APIKeyMiddleware: apiKeyMiddleware,
		KnowledgeHandler: knowledgeHandler,
		ChatHandler:      chatHandler,
	}

	routeConfig.Setup()

	config.App.Get("*", func(c *fiber.Ctx) error {
		return c.Status(404).JSON(fiber.Map{
			"code":    404,
			"message": "what are you looking for",
		})
	})
}
