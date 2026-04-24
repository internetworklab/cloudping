package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	pkgbitmap "github.com/internetworklab/cloudping/pkg/bitmap"
	pkgbot "github.com/internetworklab/cloudping/pkg/bot"
	pkgtui "github.com/internetworklab/cloudping/pkg/tui"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

type ProbeCLI struct {
	From string `short:"s" name:"from" help:"Specify the source node for originating packets"`
	CIDR string `arg:"" name:"cidr" help:"CIDR of the subnet to probe, e.g. 172.23.0.0/24"`
}

type ProbeHandler struct {
	// Name of fonts to search
	FontNames []string

	LocationsProvider pkgtui.LocationsProvider

	ProbeEventsProvider pkgtui.ProbeEventsProvider

	MaxBitsizeAllowed *int

	ConversationManager *pkgbot.ConversationManager
}

func (handler *ProbeHandler) getMaxBitsize() int {
	if x := handler.MaxBitsizeAllowed; x != nil {
		return *x
	}
	const defaultMaxBitSize = 20
	return defaultMaxBitSize
}

func (handler *ProbeHandler) getConversationManager() (*pkgbot.ConversationManager, error) {
	if handler.ConversationManager == nil {
		return nil, errors.New("ConversationManager is not provided")
	}
	return handler.ConversationManager, nil
}

func (handler *ProbeHandler) getLocationsProvider() (pkgtui.LocationsProvider, error) {
	if handler.LocationsProvider == nil {
		return nil, errors.New("Locations provider is not provided")
	}
	return handler.LocationsProvider, nil
}

func (handler *ProbeHandler) getEVsProvider() (pkgtui.ProbeEventsProvider, error) {
	if handler.ProbeEventsProvider == nil {
		return nil, errors.New("Events provider is not provided")
	}
	return handler.ProbeEventsProvider, nil
}

func (handler *ProbeHandler) GetUsage() string {
	return "[-s|--from <source_node_id>] <cidr>"
}

func (handler *ProbeHandler) parseCLIString(cliString string) (*ProbeCLI, error) {
	// Buffer for storing help text
	helpBuff := &strings.Builder{}

	cliSegs := strings.Fields(cliString)
	if len(cliSegs) > 0 && strings.HasPrefix(cliSegs[0], "/") {
		// strip the first /-leading segment
		cliSegs = cliSegs[1:]
	}

	if len(cliSegs) == 0 {
		return nil, errors.New("no arguments provided")
	}

	probeCLI := &ProbeCLI{}
	exitCh := make(chan int, 1)
	defer close(exitCh)

	kongInstance := kong.Must(
		probeCLI,
		kong.Writers(helpBuff, helpBuff),
		kong.Name(""),
		kong.Exit(func(code int) {
			exitCh <- code
		}),
	)

	getHelp := func() string {
		select {
		case <-exitCh:
			return helpBuff.String()
		default:
			return ""
		}
	}

	_, err := kongInstance.Parse(cliSegs)
	if err != nil {
		fmt.Fprintf(helpBuff, "Error: %v", err)
		if help := getHelp(); help != "" {
			return nil, fmt.Errorf("Help:\n%s\n", help)
		} else {
			return nil, fmt.Errorf("Unknown error: %v", err)
		}
	}

	if help := getHelp(); help != "" {
		return nil, fmt.Errorf("Help:\n%s\n", help)
	}

	return probeCLI, nil
}

func (handler *ProbeHandler) getFontNames() []string {
	var defaultFontNames = []string{"Noto Sans Mono", "monospace"}
	if handler == nil || len(handler.FontNames) == 0 {
		return defaultFontNames
	}
	return handler.FontNames
}

func (handler *ProbeHandler) getGridSize(bits int) int {
	if bits >= 0 && bits < 13 {
		// bits = 0,1,2,...,10,11,12
		return 32
	} else if bits >= 13 && bits < 17 {
		// bits = 13, 14, 15, 16
		return 16
	} else if bits >= 17 && bits < 21 {
		// bits = 17, 18, 19, 20
		return 8
	} else {
		// bits >= 21
		return 4
	}
}

const inProgressText = "In progress ..."

func (handler *ProbeHandler) HandleProbeCancelQueryCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	convMngr, err := handler.getConversationManager()
	if err != nil {
		log.Panic(err)
	}

	chatId := update.CallbackQuery.Message.Message.Chat.ID
	msgId := update.CallbackQuery.Message.Message.ID

	conversationKey := &pkgbot.ConversationKey{
		ChatId: chatId,
		MsgId:  msgId,
	}
	ctx, canceller := context.WithCancel(ctx)
	defer canceller()

	if _, err := convMngr.CutIn(ctx, conversationKey, nil); err != nil {
		log.Printf("Failed to cancel conversation: %v", err)
	}

	newCaption := "Cancelled"
	if msg := update.CallbackQuery.Message.Message; msg != nil {
		newCaption = strings.Replace(msg.Caption, inProgressText, "Cancelled", 1)
	}

	_, err = b.EditMessageCaption(ctx, &bot.EditMessageCaptionParams{
		ChatID:    chatId,
		MessageID: msgId,
		Caption:   newCaption,
	})
	if err != nil {
		log.Printf("failed to edit message %d in chat %d: %v", msgId, chatId, err)
	}
}

func (handler *ProbeHandler) HandleProbe(ctx context.Context, b *bot.Bot, update *models.Update) {

	if update.Message == nil {
		return
	}

	ctx, canceller := context.WithCancel(ctx)
	defer canceller()

	sessMngr, err := handler.getConversationManager()
	if err != nil {
		log.Panic(err)
	}

	chatId := update.Message.Chat.ID
	msgId := update.Message.ID

	msgRecord := pkgbot.MessageRecord{
		DateTime: time.Now(),
		Content:  update.Message.Text,
	}

	replyParams := &models.ReplyParameters{ChatID: chatId, MessageID: msgId}

	LogCommand(update, update.Message.Text)

	sendText := func(text string) {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          chatId,
			Text:            text,
			ReplyParameters: replyParams,
		})
	}

	locsProvider, err := handler.getLocationsProvider()
	if err != nil {
		sendText(fmt.Sprintf("Can't get location provider: %s", err.Error()))
		return
	}

	allLocs, err := locsProvider.GetAllLocations(ctx)
	if err != nil {
		sendText(fmt.Sprintf("Can't get locations: %s", err.Error()))
		return
	}

	cliString := update.Message.Text
	probeCLI, err := handler.parseCLIString(cliString)
	if err != nil {
		helpText := err.Error()
		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          chatId,
			Text:            helpText,
			ReplyParameters: replyParams,
			Entities: []models.MessageEntity{
				{
					Type:   models.MessageEntityTypePre,
					Offset: 0,
					Length: len(helpText),
				},
			},
		})
		log.Printf("failed to send help message: %v", err)
		return
	}

	if probeCLI.From == "" {
		if len(allLocs) == 0 {
			sendText("No source nodes available. See /list for available nodes.")
			return
		}
		probeCLI.From = allLocs[rand.IntN(len(allLocs))].Id
	} else {
		idx := slices.IndexFunc(allLocs, func(elem pkgtui.LocationDescriptor) bool {
			return elem.Id == probeCLI.From
		})
		if idx == -1 {
			sendText(fmt.Sprintf("Invalid source node %q.\nSee /list for available nodes and specify the source node with --from=<node_id>", probeCLI.From))
			return
		}
	}

	_, cidrObj, err := net.ParseCIDR(probeCLI.CIDR)
	if err != nil {
		sendText(fmt.Sprintf("Failed to parse CIDR %s: %s", probeCLI.CIDR, err.Error()))
		return
	}

	ones, bits := cidrObj.Mask.Size()
	bitSize := bits - ones // number of host bits, ones is the number of bits for network address
	maxBitSize := handler.getMaxBitsize()
	if bitSize > maxBitSize {
		sendText(fmt.Sprintf("CIDR %s is too large, maximum bit size is %d", probeCLI.CIDR, maxBitSize))
		return
	}

	numSamples := uint32(1) << uint32(bitSize)
	evsProvider, err := handler.getEVsProvider()
	if err != nil {
		sendText(fmt.Sprintf("Failed to get probe events provider: %s", err.Error()))
		return
	}

	buttons := make([][]models.InlineKeyboardButton, 1)
	buttons[0] = make([]models.InlineKeyboardButton, 1)
	buttons[0][0] = models.InlineKeyboardButton{
		Text:         "Cancel",
		CallbackData: "probe_cancel",
	}
	buttonsMarkup := &models.InlineKeyboardMarkup{InlineKeyboard: buttons}

	gridCellSize := handler.getGridSize(bitSize)
	sendImg := func(ctx context.Context, probed int, total int, lastMsgId *int, rttMs []int) *int {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		imgFilename, err := pkgbitmap.RenderProbeHeatmap(
			rttMs,
			uint32(gridCellSize),
			probed,
			*cidrObj,
			handler.getFontNames(),
		)

		if err != nil {
			sendText(err.Error())
			return nil
		}
		defer os.Remove(imgFilename)
		imgFile, err := os.Open(imgFilename)
		if err != nil {
			log.Panic(err)
		}
		defer imgFile.Close()
		imgFileUp := models.InputFileUpload{Filename: imgFilename, Data: imgFile}

		reachables := probed
		for i, x := range rttMs {
			if x < 0 && i < probed {
				reachables--
			}
		}

		captionBuf := strings.Builder{}
		fmt.Fprintf(&captionBuf, "Scan report of %s\n", cidrObj.String())
		fmt.Fprintf(&captionBuf, "Source: %s\n", probeCLI.From)
		fmt.Fprintf(&captionBuf, "Probed: %d / %d\n", probed, total)
		fmt.Fprintf(&captionBuf, "Reachable: %d / %d\n", reachables, probed)

		replyMarkup := buttonsMarkup
		if probed == total {
			replyMarkup = &models.InlineKeyboardMarkup{
				InlineKeyboard: make([][]models.InlineKeyboardButton, 0),
			}
		} else {
			fmt.Fprintf(&captionBuf, inProgressText)
		}

		captionText := captionBuf.String()

		if lastMsgId != nil {
			_, err := b.EditMessageMedia(ctx, &bot.EditMessageMediaParams{
				ChatID:    chatId,
				MessageID: *lastMsgId,
				Media: &models.InputMediaPhoto{
					Media:           fmt.Sprintf("attach://%s", filepath.Base(imgFilename)),
					MediaAttachment: imgFile,
					Caption:         captionText,
				},
				ReplyMarkup: replyMarkup,
			})
			if err != nil {
				log.Printf("failed to edit message %d in chat %d: %v", *lastMsgId, chatId, err)
				return lastMsgId
			}
			return lastMsgId
		} else {
			msg, err := b.SendPhoto(ctx, &bot.SendPhotoParams{
				ChatID:          chatId,
				Photo:           &imgFileUp,
				ReplyParameters: replyParams,
				Caption:         captionText,
				ReplyMarkup:     replyMarkup,
			})
			if err != nil {
				log.Printf("failed to send probe response: %v", err)
				return nil
			}
			msgId := msg.ID

			return &msgId
		}
	}

	type probeResultT struct {
		RTTMs  []int
		Probed int
	}
	rttMsChan := make(chan probeResultT, 1)

	go func(ctx context.Context, evsProvider pkgtui.ProbeEventsProvider) {
		cidr := *cidrObj
		evsChan := evsProvider.GetProbeEvents(ctx, pkgtui.ProbeRequestDescriptor{
			FromNodeId: probeCLI.From,
			TargetCIDR: cidr,
		})
		probeResult := &probeResultT{}
		rttMsChan <- *probeResult
		rttMs := make([]int, numSamples)
		probed := 0
		defer func() {
			rttMsChan <- *probeResult
			close(rttMsChan)
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-evsChan:
				if !ok {
					return
				}
				if err := ev.Err; err != nil {
					log.Printf("Got error from upstream: %v", err)
					return
				}
				offset := pkgutils.GetOffset(cidr, ev.IP)
				if offset >= 0 && offset < uint64(len(rttMs)) {
					rttMs[offset] = ev.RTTMs
					probed++

					probeResult.Probed = probed
					probeResult.RTTMs = make([]int, len(rttMs))
					copy(probeResult.RTTMs, rttMs)
				}
			case rttMsChan <- *probeResult:
			}
		}
	}(ctx, evsProvider)

	const mediaMsgEditIntv time.Duration = 10 * time.Second
	ticker := time.NewTicker(mediaMsgEditIntv)
	defer ticker.Stop()

	var lastMsgId *int = nil
	rttMs := make([]int, numSamples)
	var probed *int = new(int)
	*probed = 0
	lastMsgId = sendImg(ctx, *probed, int(numSamples), lastMsgId, rttMs)
	if lastMsgId != nil {
		conversationKey := &pkgbot.ConversationKey{
			ChatId: chatId,
			MsgId:  *lastMsgId,
		}

		// Register the Canceller for checking in, so that the user can cancell it later.
		if err := sessMngr.CheckIn(ctx, conversationKey, msgRecord, canceller); err != nil {
			log.Panic(err)
		}
	}

	defer func(ctx context.Context) {
		if *probed > 0 {
			<-time.After(mediaMsgEditIntv)
			lastMsgId = sendImg(ctx, *probed, int(numSamples), lastMsgId, rttMs)
		}
	}(ctx)

	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
			lastMsgId = sendImg(ctx, *probed, int(numSamples), lastMsgId, rttMs)
		case probeResult, ok := <-rttMsChan:
			if !ok {
				return
			}
			if probeResult.Probed == 0 || probeResult.RTTMs == nil {
				continue
			}
			if len(probeResult.RTTMs) != len(rttMs) {
				log.Panicf("RTT slice length mismatch! %d != %d", len(rttMs), len(probeResult.RTTMs))
			}
			copy(rttMs, probeResult.RTTMs)
			*probed = probeResult.Probed
		}
	}
}
