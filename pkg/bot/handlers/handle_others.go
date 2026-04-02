package handlers

import (
	"context"
	"log"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func HandleStart(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message != nil {
		if update.Message.Chat.Type == models.ChatTypePrivate {
			_, err := b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "Already started!",
			})
			if err != nil {
				log.Printf("failed to send message: %v", err)
			}
		}
	}
}
