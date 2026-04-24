package handlers

import (
	"context"
	"encoding/json"
	"log"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

type VersionHandler struct {
	Version *pkgutils.BuildVersion
}

func (handler *VersionHandler) HandleVersion(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	bv := handler.Version

	versionJSON, err := json.MarshalIndent(bv, "", "  ")
	if err != nil {
		log.Printf("failed to marshal build version: %v", err)
		return
	}

	txt := string(versionJSON)
	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   txt,
		Entities: []models.MessageEntity{
			{
				Type:   models.MessageEntityTypePre,
				Offset: 0,
				Length: len(txt),
			},
		},
	})
	if err != nil {
		log.Printf("failed to send version message: %v", err)
	}
}
