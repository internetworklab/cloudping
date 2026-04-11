package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/alecthomas/kong"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	pkgbitmap "github.com/internetworklab/cloudping/pkg/bitmap"
	pkgbot "github.com/internetworklab/cloudping/pkg/bot"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

const defaultGridCellSize = 32

type ProbeCLI struct {
	From string `name:"from" help:"Specify the node that originate packets"`
	CIDR string `arg:"" name:"cidr" help:"CIDR of the subnet to probe, e.g. 172.23.0.0/24"`
}

type ProbeHandler struct {
	// Name of fonts to search
	FontNames []string
}

func (prober *ProbeHandler) GetUsage() string {
	return "/probe <from> <cidr>"
}

func (prober *ProbeHandler) parseCLIString(cliString string) (*ProbeCLI, *kong.Context, error) {
	cliSegs := pkgutils.SplitBySpace(cliString)
	if len(cliSegs) == 0 {
		return nil, nil, errors.New("no arguments provided")
	}

	pingCLI := new(ProbeCLI)
	parser, err := kong.New(pingCLI)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create kong parser: %w", err)
	}

	kongCtx, err := parser.Parse(cliSegs)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse ping CLI: %w", err)
	}

	return pingCLI, kongCtx, nil
}

func (prober *ProbeHandler) getFontNames() []string {
	var defaultFontNames = []string{"Noto Sans Mono", "monospace"}
	if prober == nil || len(prober.FontNames) == 0 {
		return defaultFontNames
	}
	return prober.FontNames
}

func (prober *ProbeHandler) HandleProbe(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	cliString := pkgbot.TrimCommandPrefix(update.Message.Text)
	probeCLI, _, err := prober.parseCLIString(cliString)

	chatId := update.Message.Chat.ID
	msgId := update.Message.ID
	replyParams := &models.ReplyParameters{ChatID: chatId, MessageID: msgId}

	imgFilename, err := pkgbitmap.GenerateRandomRGBAPNGBitmap(
		defaultGridCellSize,
		probeCLI.CIDR,
		prober.getFontNames(),
	)

	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          chatId,
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
		ChatID:          chatId,
		Photo:           &imgFileUp,
		ReplyParameters: replyParams,
	})
	if err != nil {
		log.Printf("failed to send probe response: %v", err)
	}
}
