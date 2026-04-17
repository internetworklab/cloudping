package handler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"strings"

	"github.com/jhillyerd/enmime"

	"golang.org/x/net/html"
)

// The purpose of email http middleware is to parse plaintext or HTML email content,
// and extract possible CLI args from it (if there is), and pass the extracted CLI args
// to next handler in the chain of handlers(middlewares) via Context value.

type WithEmailHandler struct {
	Next http.Handler
}

func (handler *WithEmailHandler) parseCLIArgs(mail *enmime.Envelope) []string {
	if rawText := mail.Text; len(rawText) > 0 {
		// fields from raw text
		rawFields := strings.Fields(rawText)
		if len(rawFields) > 0 {
			return rawFields
		}
	}

	if htmlText := mail.HTML; len(htmlText) > 0 {
		// Parsing html doc tree
		htmlReader := strings.NewReader(mail.HTML)
		doc, err := html.Parse(htmlReader)
		if err != nil {
			log.Panic(err)
		}
		htmlFields := make([]string, 0)
		for elem := range doc.Descendants() {
			if elem.Type == html.TextNode {
				if trimed := strings.TrimSpace(elem.Data); len(trimed) > 0 {
					htmlFields = append(htmlFields, strings.Fields(trimed)...)
				}
			}
		}
		return htmlFields
	}

	return make([]string, 0)
}

type CtxKeyWithEmail string

const CtxKeyWithEmailParsedEmail CtxKeyWithEmail = "with_email_parsed_email"

type ParsedEmail struct {
	From    string
	To      []*mail.Address
	Subject string
	Args    []string
}

func (handler *WithEmailHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	// Parse message body with enmime.
	mail, err := enmime.ReadEnvelope(r.Body)
	if err != nil {
		writeErrorResponse(w, fmt.Sprintf("Invalid email format: %s", err.Error()), http.StatusBadRequest)
		return
	}

	// Headers can be retrieved via Envelope.GetHeader(name).
	mailFrom := mail.GetHeader("From")
	log.Printf("Got email From: %v\n", mailFrom)

	// Address-type headers can be parsed into a list of decoded mail.Address structs.
	alist, _ := mail.AddressList("To")
	for _, addr := range alist {
		log.Printf("Got email To: %s <%s>\n", addr.Name, addr.Address)
	}

	// enmime can decode quoted-printable headers.
	mailSubj := mail.GetHeader("Subject")
	log.Printf("Got email Subject: %v\n", mailSubj)

	cliArgs := handler.parseCLIArgs(mail)
	if len(cliArgs) == 0 {
		writeErrorResponse(w, "Empty command, no CLI args were parsed", http.StatusBadRequest)
		return
	}

	parsedEmail := &ParsedEmail{
		From:    mailFrom,
		To:      alist,
		Subject: mailSubj,
		Args:    cliArgs,
	}

	ctx := r.Context()
	ctx = context.WithValue(ctx, CtxKeyWithEmailParsedEmail, parsedEmail)
	r = r.WithContext(ctx)
	handler.Next.ServeHTTP(w, r)
}
