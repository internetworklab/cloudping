package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	pkgauth "github.com/internetworklab/cloudping/pkg/auth"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

type BasicProfileHandler struct {
	TokenValidator pkgauth.JWTValidator
}

type BasicProfile struct {
	SubjectId string `json:"subject_id"`
	Username  string `json:"username"`
}

func (h *BasicProfileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := extractJWTFromRequest(r)
	registeredClaims, customClaimsAny, err := h.TokenValidator.ParseToken(ctx, token)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Sprintf("invalid token: %v", err)})
		return
	}
	if registeredClaims == nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: "invalid token: can't parse as registered claims"})
		return
	}

	basicProfile := &BasicProfile{
		SubjectId: registeredClaims.ID,
	}
	if customClaimsAny != nil {
		if customClaims, ok := customClaimsAny.(*pkgauth.CustomClaimType); ok && customClaims != nil {
			basicProfile.Username = customClaims.Username
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(basicProfile)
}
