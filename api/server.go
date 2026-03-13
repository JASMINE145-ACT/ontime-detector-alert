package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"ontime-detector-alert/alerts"
)

type Server struct {
	repo alerts.Repository
	mux  *http.ServeMux
}

func NewServer(repo alerts.Repository) *Server {
	s := &Server{
		repo: repo,
		mux:  http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/alerts", s.handleAlerts)
	s.mux.HandleFunc("/alerts/", s.handleAlertByID)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type createAlertRequest struct {
	Symbol          string  `json:"symbol"`
	Direction       string  `json:"direction"`
	Threshold       float64 `json:"threshold"`
	UserID          string  `json:"user_id"`
	CooldownSeconds int     `json:"cooldown_seconds"`
}

func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req createAlertRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if req.Symbol == "" || req.Direction == "" || req.UserID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "symbol, direction and user_id are required"})
			return
		}
		dir := alerts.Direction(req.Direction)
		if dir != alerts.DirectionAbove && dir != alerts.DirectionBelow {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "direction must be 'above' or 'below'"})
			return
		}
		a := &alerts.Alert{
			Symbol:          req.Symbol,
			Direction:       dir,
			Threshold:       req.Threshold,
			UserID:          req.UserID,
			Active:          true,
			CooldownSeconds: req.CooldownSeconds,
		}
		if err := s.repo.Create(a); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create alert"})
			return
		}
		writeJSON(w, http.StatusCreated, a)
	case http.MethodGet:
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user_id is required"})
			return
		}
		alertsList, err := s.repo.ListByUser(userID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list alerts"})
			return
		}
		writeJSON(w, http.StatusOK, alertsList)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAlertByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/alerts/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}
	if err := s.repo.Delete(id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "alert not found"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

