package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/alecthomas/kong"
)

type TracerouteCLI struct {
	IPv4        bool   `short:"4" name:"prefer-ipv4" help:"Use IPv4"`
	IPv6        bool   `short:"6" name:"prefer-ipv6" help:"Use IPv6"`
	Count       int    `short:"c" name:"count" help:"Number of packets to send" default:"24"`
	Destination string `arg:"" name:"destination" help:"Destination to trace"`
}

func (cmd *TracerouteCLI) Run() error {
	fmt.Printf("TracerouteCLI.Run() is called:\n")
	json.NewEncoder(os.Stdout).Encode(cmd)
	return nil
}

func main() {

	// Buffer for storing help text
	helpBuff := &strings.Builder{}

	cli := &TracerouteCLI{}
	exitCh := make(chan int, 1)
	kongInstance := kong.Must(
		cli,
		kong.Writers(helpBuff, helpBuff),
		kong.Name(""),
		kong.Exit(func(code int) {
			exitCh <- code
		}),
	)

	getHelp := func() string {
		select {
		case <-exitCh:
			return fmt.Sprintf("Help:\n%s", helpBuff.String())
		default:
			return ""
		}
	}

	fmt.Printf("os.Args[1:]:\n")
	json.NewEncoder(os.Stdout).Encode(os.Args[1:])

	kongCtx, err := kongInstance.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintf(helpBuff, "Error: %v", err)
		fmt.Print(getHelp())
		return
	}

	if help := getHelp(); help != "" {
		fmt.Print(help)
		return
	}

	fmt.Printf("Full Path: %s\n", kongCtx.Value(kongCtx.Path[0]))

	if err := kongCtx.Run(); err != nil {
		log.Printf("Running error: %v", err)
		return
	}
}
