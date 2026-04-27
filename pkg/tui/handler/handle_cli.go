package handler

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/alecthomas/kong"
	pkgtui "github.com/internetworklab/cloudping/pkg/tui"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

type CLICtx struct {
	ResponseWriter      http.ResponseWriter
	Request             *http.Request
	LocationsProvider   pkgtui.LocationsProvider
	PingEventsProvider  pkgtui.PingEventsProvider
	GlobalSharedContext *pkgutils.GlobalSharedContext
}

type CLI struct {
	List       ListCMD       `cmd:"" name:"list" help:"List all available nodes"`
	Ping       PingCMD       `cmd:"" name:"ping" help:"Ping"`
	Version    VersionCMD    `cmd:"" name:"version" help:"Show the build version of the instance"`
	Traceroute TracerouteCMD `cmd:"" name:"traceroute" help:"Traceroute"`
}

type CLIHandler struct {
	LocationsProvider   pkgtui.LocationsProvider
	PingEventsProvider  pkgtui.PingEventsProvider
	GlobalSharedContext *pkgutils.GlobalSharedContext
}

func (handler *CLIHandler) getCLIArgsFromRawBody(r *http.Request) ([]string, error) {
	defer r.Body.Close()
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(rawBody)), nil
}

func (handler *CLIHandler) getCLIArgsAnyway(r *http.Request) ([]string, error) {
	ctx := r.Context()
	parsedMailAny := ctx.Value(CtxKeyWithEmailParsedEmail)
	if parsedMailAny != nil {
		parsedMail := parsedMailAny.(*ParsedEmail)
		return parsedMail.Args, nil
	}

	parsedCLIArgs, err := handler.getCLIArgsFromRawBody(r)
	if err != nil {
		return nil, err
	}
	if len(parsedCLIArgs) > 0 {
		return parsedCLIArgs, nil
	}

	return nil, errors.New("no cli args were parsed")
}

func (handler *CLIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cliArgs, err := handler.getCLIArgsAnyway(r)
	if err != nil {
		writeErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	globalCtx := &CLICtx{
		ResponseWriter:      w,
		Request:             r,
		LocationsProvider:   handler.LocationsProvider,
		PingEventsProvider:  handler.PingEventsProvider,
		GlobalSharedContext: handler.GlobalSharedContext,
	}

	cli := &CLI{}

	exitCh := make(chan int, 1)
	doneCh := make(chan struct{})

	piped, out := io.Pipe()
	defer out.Close()

	go func() {
		sb := &strings.Builder{}
		io.Copy(sb, piped)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(sb.String()))
		close(doneCh)
	}()

	kongInstance := kong.Must(
		cli,
		kong.Writers(out, out),
		kong.Name(""),
		kong.Exit(func(code int) {
			exitCh <- code
		}),
	)

	kongCtx, err := kongInstance.Parse(cliArgs)
	if err != nil {
		fmt.Fprintf(out, fmt.Sprintf("Error: %v", err))
	}

	select {
	case <-exitCh:
		out.Close()
		<-doneCh
		return
	default:
	}

	err = kongCtx.Run(globalCtx)
	if err != nil {
		log.Printf("tui error: %s", err.Error())
	}
}
