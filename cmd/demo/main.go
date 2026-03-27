package main

import (
	"log"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/joho/godotenv"
	"github.com/wneessen/go-mail"
)

type SMTPSendRequest struct {
	// e.g. alice@example.com
	FromAddr string

	// e.g. bob@example.com
	ToAddr string

	// Subject (title) of the email
	Subject string

	// Body of the email
	Content string
}

func DoSend(sendRequest *SMTPSendRequest, client *mail.Client) error {
	message := mail.NewMsg()
	if err := message.From(sendRequest.FromAddr); err != nil {
		log.Fatalf("failed to set FROM address: %s", err)
	}
	if err := message.To(sendRequest.ToAddr); err != nil {
		log.Fatalf("failed to set TO address: %s", err)
	}
	message.Subject(sendRequest.Subject)
	message.SetBodyString(mail.TypeTextPlain, sendRequest.Content)

	return client.DialAndSend(message)
}

const defaultIDLEWakeUpIntv time.Duration = 30 * time.Second

func Receive(c *imapclient.Client, watchingInboxes []string) error {

	for _, inbox := range watchingInboxes {

		if _, err := c.Select(inbox, &imap.SelectOptions{ReadOnly: true}).Wait(); err != nil {
			log.Printf("Failed to select mailbox %s: %s", inbox, err.Error())
			continue
		}

		supFlgs := make([]string, 0)
		for _, flg := range c.Mailbox().Flags {
			supFlgs = append(supFlgs, string(flg))
		}
		log.Printf("Looking at inbox %s, supported flags: %q ...", inbox, strings.Join(supFlgs, ","))

		if slices.IndexFunc(c.Mailbox().Flags, func(flg imap.Flag) bool { return flg == imap.FlagSeen }) == -1 {
			log.Printf("Inbox %s does not support the SEEN flag", inbox)
			continue
		}

		searchData, err := c.UIDSearch(&imap.SearchCriteria{NotFlag: []imap.Flag{imap.FlagSeen}}, nil).Wait()
		if err != nil {
			log.Printf("failed to search inbox %s: %s", inbox, err.Error())
			continue
		}

		fetchBuf, err := c.Fetch(searchData.All, &imap.FetchOptions{Envelope: true}).Collect()
		if err != nil {
			log.Printf("failed to fetch emails in %s: %s", inbox, err.Error())
			continue
		}
		for _, msg := range fetchBuf {
			senders := make([]string, 0)
			for _, sd := range msg.Envelope.Sender {
				senders = append(senders, sd.Addr())
			}

			replyTo := make([]string, 0)
			for _, sd := range msg.Envelope.ReplyTo {
				replyTo = append(replyTo, sd.Addr())
			}

			froms := make([]string, 0)
			for _, sd := range msg.Envelope.From {
				froms = append(froms, sd.Addr())
			}

			log.Printf("Found message in %s: subject=%q, date=%q, senders=%q, replyTo=%q, froms=%q", inbox, msg.Envelope.Subject, msg.Envelope.Date, strings.Join(senders, ","), strings.Join(replyTo, ","), strings.Join(froms, ","))
		}
	}

	return nil
}

func main() {
	godotenv.Load()

	imapServerPort := os.Getenv("IMAP_SERVERPORT")
	// smtpHost := os.Getenv("SMTP_HOST")
	imapUsername := os.Getenv("IMAP_USERNAME")
	imapPasswd := os.Getenv("IMAP_PASSWORD")
	// smtpUsername := os.Getenv("SMTP_USER")
	// smtpPasswd := os.Getenv("SMTP_PASS")
	//

	c, err := imapclient.DialTLS(imapServerPort, nil)
	if err != nil {
		log.Fatalf("failed to dial IMAP server: %v", err)
	}
	defer c.Close()
	log.Printf("Dialed to IMAP server %s", imapServerPort)

	capSet, err := c.Capability().Wait()
	if err != nil {
		log.Fatalf("failed to get capability: %v", err)
	}
	if !capSet.Has(imap.CapIdle) {
		log.Fatalf("server does not support IDLE")
	}

	if err := c.Login(imapUsername, imapPasswd).Wait(); err != nil {
		log.Fatalf("failed to login: %v", err)
	}
	log.Printf("Logged in to IMAP server")

	if err := Receive(c, []string{"INBOX", "Junk E-Mail"}); err != nil {
		log.Fatal(err)
	}

	// Deliver the mails via SMTP
	// client, err := mail.NewClient(smtpHost,
	// 	mail.WithSMTPAuth(mail.SMTPAuthAutoDiscover), mail.WithTLSPortPolicy(mail.TLSMandatory),
	// 	mail.WithUsername(smtpUsername), mail.WithPassword(smtpPasswd),
	// )
	// if err != nil {
	// 	log.Fatalf("failed to create new mail delivery client: %s", err)
	// }
	// DoSend(&SMTPSendRequest{}, client)
}
