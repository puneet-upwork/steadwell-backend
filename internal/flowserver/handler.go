package flowserver

import (
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"steadwell/internal/flows"
	telegramflow "steadwell/internal/flows/telegram"
	"steadwell/internal/telegramapi"
)

const telegramSecretHeader = "X-Telegram-Bot-Api-Secret-Token"

func RegisterRoutes(r *gin.Engine, deps *flows.Deps, telegramSecret string) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	r.POST("/webhook/telegram", handleTelegram(deps, telegramSecret))
}

func handleTelegram(deps *flows.Deps, secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if secret != "" && c.GetHeader(telegramSecretHeader) != secret {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid telegram secret token"})
			return
		}
		raw, err := io.ReadAll(c.Request.Body)
		if err != nil {
			slog.Error("telegram webhook read body", "error", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
			return
		}
		slog.Info("telegram webhook", "bytes", len(raw), "remote", c.ClientIP())
		update, err := telegramapi.ParseJSON(raw)
		if err != nil {
			slog.Warn("telegram webhook invalid json", "error", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}
		if update.Message == nil && update.CallbackQuery == nil {
			slog.Warn("telegram webhook missing message", "update_id", update.UpdateID)
			c.JSON(http.StatusBadRequest, gin.H{"error": "expected telegram update"})
			return
		}
		result, err := telegramflow.Run(c.Request.Context(), deps, update)
		if err != nil {
			slog.Error("telegram webhook run", "error", err, "update_id", update.UpdateID)
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, result)
	}
}
