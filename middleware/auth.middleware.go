package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"support-agent-golang/helper"
)

// NewAPIKeyMiddleware protects the API with a shared secret passed in
// the X-API-Key header. When api.key in config.json is empty the check
// is skipped (open mode, useful during local development).
// It also reads X-Lang (ID default) so responses honour the caller locale.
func NewAPIKeyMiddleware(log *logrus.Logger, config *viper.Viper) fiber.Handler {
	apiKey := config.GetString("api.key")

	return func(ctx *fiber.Ctx) error {
		ctx.Locals("lang", ctx.Get("X-Lang", "ID"))

		if apiKey != "" && ctx.Get("X-API-Key") != apiKey {
			log.Warn("unauthorized request: invalid or missing X-API-Key")
			return ctx.Status(401).JSON(helper.Response(ctx, config, 401, "", helper.EmptyObject()))
		}

		return ctx.Next()
	}
}
