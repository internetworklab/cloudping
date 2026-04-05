package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/alecthomas/kong"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	pkgbitmap "github.com/internetworklab/cloudping/pkg/bitmap"
	pkgbot "github.com/internetworklab/cloudping/pkg/bot"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

const defaultGridCellSize = 32

type ProbeCLI struct {
	From string `short:"s" name:"from" help:"Specify the source node for originating packets"`
	CIDR string `arg:"" name:"cidr" help:"CIDR of the subnet to probe, e.g. 172.23.0.0/24"`
}

type ProbeHandler struct {
	// Name of fonts to search
	FontNames []string

	LocationsProvider pkgbot.LocationsProvider

	ProbeEventsProvider pkgbot.ProbeEventsProvider

	MaxBitsizeAllowed *int
}

func (handler *ProbeHandler) getMaxBitsize() int {
	if x := handler.MaxBitsizeAllowed; x != nil {
		return *x
	}
	const defaultMaxBitSize = 10
	return defaultMaxBitSize
}

func (handler *ProbeHandler) getLocationsProvider() (pkgbot.LocationsProvider, error) {
	if handler.LocationsProvider == nil {
		return nil, errors.New("Locations provider is not provided")
	}
	return handler.LocationsProvider, nil
}

func (handler *ProbeHandler) getEVsProvider() (pkgbot.ProbeEventsProvider, error) {
	if handler.ProbeEventsProvider == nil {
		return nil, errors.New("Events provider is not provided")
	}
	return handler.ProbeEventsProvider, nil
}

func (handler *ProbeHandler) GetUsage() string {
	return "/probe -s <source_node_id> <cidr>"
}

func (handler *ProbeHandler) parseCLIString(cliString string) (*ProbeCLI, *kong.Context, error) {

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
		return nil, nil, fmt.Errorf("failed to parse CLI: %w", err)
	}

	return pingCLI, kongCtx, nil
}

func (handler *ProbeHandler) getFontNames() []string {
	var defaultFontNames = []string{"Noto Sans Mono", "monospace"}
	if handler == nil || len(handler.FontNames) == 0 {
		return defaultFontNames
	}
	return handler.FontNames
}

func (handler *ProbeHandler) HandleProbe(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatId := update.Message.Chat.ID
	msgId := update.Message.ID
	replyParams := &models.ReplyParameters{ChatID: chatId, MessageID: msgId}

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

	cliString := pkgbot.TrimCommandPrefix(update.Message.Text)
	probeCLI, _, err := handler.parseCLIString(cliString)
	if err != nil {
		sendText(fmt.Sprintf("Error: %s\nUsage: %s", err.Error(), handler.GetUsage()))
		return
	}

	idx := slices.IndexFunc(allLocs, func(elem pkgbot.LocationDescriptor) bool {
		return elem.Id == probeCLI.From
	})
	if idx == -1 {
		sendText(fmt.Sprintf("Empty or invalid source node %q.\nSee /list for available nodes and specify the source node with --from=<node_id>", probeCLI.From))
		return
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

	sendImg := func(probed int, lastMsgId *int, rttMs []int) *int {
		imgFilename, err := pkgbitmap.RenderProbeHeatmap(
			rttMs,
			defaultGridCellSize,
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

		if lastMsgId != nil {
			_, err := b.EditMessageMedia(ctx, &bot.EditMessageMediaParams{
				ChatID:    chatId,
				MessageID: *lastMsgId,
				Media: &models.InputMediaPhoto{
					Media:           fmt.Sprintf("attach://%s", filepath.Base(imgFilename)),
					MediaAttachment: imgFile,
					Caption:         fmt.Sprintf("Scan report of %s", cidrObj.String()),
				},
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
				Caption:         fmt.Sprintf("Scan report of %s", cidrObj.String()),
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

	go func(ctx context.Context, evsProvider pkgbot.ProbeEventsProvider) {
		cidr := *cidrObj
		evsChan := evsProvider.GetProbeEvents(ctx, pkgbot.ProbeRequestDescriptor{
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
	var lastMsgId *int = nil
	rttMs := make([]int, numSamples)
	var probed *int = new(int)
	*probed = 0
	defer func() {
		if *probed > 0 {
			<-time.After(mediaMsgEditIntv)
			lastMsgId = sendImg(*probed, lastMsgId, rttMs)
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			lastMsgId = sendImg(*probed, lastMsgId, rttMs)
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
