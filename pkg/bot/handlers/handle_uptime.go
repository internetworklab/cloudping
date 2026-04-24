package handlers

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type UptimeHandler struct {
	StartedAt time.Time
}

func (handler *UptimeHandler) HandleUptime(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message != nil {
		startedAt := handler.StartedAt
		uptime := time.Since(startedAt)
		txt := fmt.Sprintf("Started at: %s\nUptime: %s", startedAt.Format(time.RFC3339), uptime.String())
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
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
			log.Printf("failed to send message: %v", err)
		}
	}
}
