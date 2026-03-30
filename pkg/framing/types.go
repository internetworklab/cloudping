package framing

import (
	pkgconnreg "github.com/internetworklab/cloudping/pkg/connreg"
)

type MessagePayload struct {
	Register               *pkgconnreg.RegisterPayload               `json:"register,omitempty"`
	Echo                   *pkgconnreg.EchoPayload                   `json:"echo,omitempty"`
	AttributesAnnouncement *pkgconnreg.AttributesAnnouncementPayload `json:"attributes_announcement,omitempty"`
}
