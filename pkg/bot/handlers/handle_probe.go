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

	cidr := strings.Split(update.Message.Text, " ")

	imgFilename, err := pkgutils.GenerateRandomRGBAPNGBitmap(32, cidr[1])
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
