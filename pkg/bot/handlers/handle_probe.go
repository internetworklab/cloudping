package handlers

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

func HandleProbe(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	replyParams := &models.ReplyParameters{ChatID: update.Message.Chat.ID, MessageID: update.Message.ID}

	cidr := strings.Split(update.Message.Text, " ")

	imgFilename, err := pkgutils.GenerateRandomRGBAPNGBitmap(
		32,
		cidr[1],
		[]string{"Noto Sans Mono", "monospace"},
	)

	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			Text:            err.Error(),
			ReplyParameters: replyParams,
		})
		return
	}
	defer os.Remove(imgFilename)
	imgFile, err := os.Open(imgFilename)
	if err != nil {
		log.Panic(err)
	}
	defer imgFile.Close()
	imgFileUp := models.InputFileUpload{Filename: imgFilename, Data: imgFile}

	_, err = b.SendPhoto(ctx, &bot.SendPhotoParams{
		ChatID:          update.Message.Chat.ID,
		Photo:           &imgFileUp,
		ReplyParameters: replyParams,
	})
	if err != nil {
		log.Printf("failed to send probe response: %v", err)
	}
}
