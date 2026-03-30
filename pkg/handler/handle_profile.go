package handler

import (
	"encoding/json"
	"net/http"

	"github.com/internetworklab/cloudping/pkg/utils"
)

type ProfileHandler struct{}

type ProfileResponse struct {
	SessionID string `json:"session_id"`
	SubjectID string `json:"subject_id"`
}

func (h *ProfileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sessionID := r.Context().Value(utils.CtxKeySessionId)
	subjectID := r.Context().Value(utils.CtxKeySubjectId)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	resp := &ProfileResponse{}
	if sessionID != nil {
		resp.SessionID = sessionID.(string)
	}
	if subjectID != nil {
		resp.SubjectID = subjectID.(string)
	}
	json.NewEncoder(w).Encode(resp)
}
