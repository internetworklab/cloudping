package main

import (
	"fmt"
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

func Receive(c *imapclient.Client, personalNs *imap.NamespaceDescriptor) error {

	if personalNs == nil {
		log.Printf("Personal NS is nil, probing")
		caps, err := c.Capability().Wait()
		if err != nil {
			return fmt.Errorf("failed to get capabilities: %w", err)
		}
		if !caps.Has(imap.CapNamespace) {
			return fmt.Errorf("Personal NS is nil while NAMESPACE capability is not supported")
		}
		namespaceDesc, err := c.Namespace().Wait()
		if err != nil {
			return fmt.Errorf("failed to get namespace: %w", err)
		}
		if namespaceDesc == nil {
			return fmt.Errorf("namespace is nil")
		}
		personalNs = &namespaceDesc.Personal[0]
		log.Printf("personal namespace: prefix=%q, delim=%q", personalNs.Prefix, personalNs.Delim)
	}

	listData, err := c.List("", "*", nil).Collect()
	if err != nil {
		return fmt.Errorf("failed to list folders: %w", err)
	}
	personalFolderIdx := slices.IndexFunc(listData, func(folder *imap.ListData) bool {
		return strings.HasPrefix(personalNs.Prefix, folder.Mailbox)
	})
	if personalFolderIdx == -1 {
		return fmt.Errorf("personal folder not found")
	}
	personalFolder := listData[personalFolderIdx]
	log.Printf("personal folder: name=%q", personalFolder.Mailbox)

	if _, err := c.Select(personalFolder.Mailbox, nil).Wait(); err != nil {
		return fmt.Errorf("failed to select folder: %w", err)
	}

	searchData, err := c.UIDSearch(&imap.SearchCriteria{NotFlag: []imap.Flag{imap.FlagSeen}}, nil).Wait()
	if err != nil {
		return fmt.Errorf("failed to search folder: %w", err)
	}

	fetchResult := c.Fetch(searchData.All, &imap.FetchOptions{Envelope: true, Flags: true, InternalDate: true})
	for {
		item := fetchResult.Next()
		if item == nil {
			break
		}
		buf, err := item.Collect()
		if err != nil {
			return fmt.Errorf("failed to collect fetch item: %w", err)
		}
		if evl := buf.Envelope; evl != nil {
			fmt.Printf("Subject: %s, Date: %v\n", evl.Subject, evl.Date.Format(time.RFC3339))
		}
	}

	// // Start idling
	// idleCmd, err := c.Idle()
	// if err != nil {
	// 	log.Fatalf("IDLE command failed: %v", err)
	// }
	// defer idleCmd.Close()

	// done := make(chan error, 1)
	// go func() {
	// 	done <- idleCmd.Wait()
	// }()

	// // Wait for 30s to receive updates from the server, then stop idling
	// t := time.NewTimer(30 * time.Second)
	// defer t.Stop()
	// select {
	// case <-t.C:
	// 	if err := idleCmd.Close(); err != nil {
	// 		log.Fatalf("failed to stop idling: %v", err)
	// 	}
	// 	if err := <-done; err != nil {
	// 		log.Fatalf("IDLE command failed: %v", err)
	// 	}
	// case err := <-done:
	// 	if err != nil {
	// 		log.Fatalf("IDLE command failed: %v", err)
	// 	}
	// }
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

	options := imapclient.Options{
		UnilateralDataHandler: &imapclient.UnilateralDataHandler{
			Expunge: func(seqNum uint32) {
				log.Printf("message %v has been expunged", seqNum)
			},
			Mailbox: func(data *imapclient.UnilateralDataMailbox) {
				if data.NumMessages != nil {
					log.Printf("a new message has been received")
				}
			},
		},
	}

	c, err := imapclient.DialTLS(imapServerPort, &options)
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

	personalNs := imap.NamespaceDescriptor{
		Prefix: "INBOX/",
		Delim:  '/',
	}
	if err := Receive(c, &personalNs); err != nil {
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
