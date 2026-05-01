package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

type LogoutHandler struct {
	RedirectAfterLogout string
}

func (h *LogoutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Sprintf("method %s not allowed, use POST", r.Method)})
		return
	}

	// Clear JWT cookie
	http.SetCookie(w, &http.Cookie{
		Name:     DefaultJWTCookieKey,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	// Clear nonce cookie
	http.SetCookie(w, &http.Cookie{
		Name:     DefaultNonceCookieKey,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	redirectURL := "/"
	if h.RedirectAfterLogout != "" {
		redirectURL = h.RedirectAfterLogout
	}

	http.Redirect(w, r, redirectURL, http.StatusFound)
}
