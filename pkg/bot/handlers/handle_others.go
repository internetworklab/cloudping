package handlers

import (
	"context"
	"log"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func HandleDefault(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message != nil {
		if update.Message.Chat.Type == models.ChatTypePrivate {
			// private message
			log.Printf("Received private message from private chat %+v: %s", update.Message.Chat.Username, update.Message.Text)
			_, err := b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   update.Message.Text,
			})
			if err != nil {
				log.Printf("failed to send message: %v", err)
			}

		} else if update.Message.Chat.Type == models.ChatTypeGroup || update.Message.Chat.Type == models.ChatTypeSupergroup {
			log.Printf("Received group message from group %+v: %s", update.Message.Chat.Title, update.Message.Text)
		}
	}
}

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
