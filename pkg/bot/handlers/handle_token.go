package handlers

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JWTHandler struct {
	Secret []byte
	Issuer string
}

func (handler *JWTHandler) HandleToken(ctx context.Context, b *bot.Bot, update *models.Update) {
	secret := handler.Secret
	if secret == nil {
		panic("JWT secret is not set in the context")
	}

	issuer := handler.Issuer

	if update.Message != nil {

		LogCommand(update, update.Message.Text)

		if update.Message.Chat.Type == models.ChatTypePrivate {
			subject := getSubjectFromMessage(update.Message)
			if subject == "" {
				_, err := b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text:   "Error: Failed to get subject from message",
				})
				if err != nil {
					log.Printf("failed to send message: %v", err)
				}
				return
			}
			subject = fmt.Sprintf("telegram:@%s", subject)

			tokenId := uuid.New().String()

			tokenObject := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
				Issuer:   issuer,
				Subject:  subject,
				IssuedAt: jwt.NewNumericDate(time.Now()),
				ID:       tokenId,
			})

			tokenString, err := tokenObject.SignedString(secret)
			if err != nil {
				_, sendErr := b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text:   fmt.Sprintf("Error: Failed to sign token: %v", err.Error()),
				})
				if sendErr != nil {
					log.Printf("failed to send message: %v", sendErr)
				}
				return
			}

			_, err = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   fmt.Sprintf("Token: %s", tokenString),
			})
			if err != nil {
				log.Printf("failed to send message: %v", err)
			}
			defer log.Printf("issued token for %s, token id: %s", subject, tokenId)
		}
	}
}
