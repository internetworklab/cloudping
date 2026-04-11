package handlers

import (
	"context"
	"log"
	"os"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

func HandleProbe(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	imgFilename, err := pkgutils.GenerateRandomRGBAPNGBitmap(10, 32, "fdda:8ca4:1556:4000:a:b:c:d/128")
	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{Text: err.Error()})
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
		ChatID: update.Message.Chat.ID,
		Photo:  &imgFileUp,
	})
	if err != nil {
		log.Printf("failed to send probe response: %v", err)
	}
}
