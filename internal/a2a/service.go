package a2a

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Request struct {
	FromAgentID string         `json:"from_agent_id"`
	ToAgentID   string         `json:"to_agent_id"`
	RequestID   string         `json:"request_id"`
	Intent      string         `json:"intent"`
	Goal        string         `json:"goal"`
	Context     map[string]any `json:"context,omitempty"`
	Artifacts   map[string]any `json:"artifacts,omitempty"`
	Constraints []string       `json:"constraints,omitempty"`
}

type Response struct {
	Status         string         `json:"status"`
	OwnerAgentID   string         `json:"owner_agent_id"`
	Summary        string         `json:"summary"`
	RequiredInputs []string       `json:"required_inputs,omitempty"`
	Result         map[string]any `json:"result,omitempty"`
	TaskID         string         `json:"task_id,omitempty"`
}

type TaskRecord struct {
	TaskID       string         `json:"task_id"`
	RequestID    string         `json:"request_id"`
	FromAgentID  string         `json:"from_agent_id"`
	ToAgentID    string         `json:"to_agent_id"`
	Intent       string         `json:"intent"`
	Goal         string         `json:"goal"`
	Status       string         `json:"status"`
	OwnerAgentID string         `json:"owner_agent_id"`
	Summary      string         `json:"summary"`
	Result       map[string]any `json:"result,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

type Service struct {
	agentID string
	role    string
	mu      sync.RWMutex
	tasks   map[string]TaskRecord
}

func NewService(agentID, role string) *Service {
	if strings.TrimSpace(agentID) == "" {
		agentID = "x-automation-agent"
	}
	return &Service{
		agentID: strings.TrimSpace(agentID),
		role:    strings.TrimSpace(role),
		tasks:   make(map[string]TaskRecord),
	}
}

func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /a2a/v1/negotiate", s.handleNegotiate)
	mux.HandleFunc("POST /a2a/v1/tasks", s.handleTaskCreate)
	mux.HandleFunc("GET /a2a/v1/tasks/{task_id}", s.handleTaskGet)
	return mux
}

func (s *Service) handleNegotiate(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeRequest(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.Negotiate(req))
}

func (s *Service) handleTaskCreate(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeRequest(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusAccepted, s.CreateTask(req))
}

func (s *Service) handleTaskGet(w http.ResponseWriter, r *http.Request) {
	record, ok := s.GetTask(strings.TrimSpace(r.PathValue("task_id")))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *Service) Negotiate(req Request) Response {
	text := strings.ToLower(strings.Join([]string{req.Intent, req.Goal, stringify(req.Context)}, " "))
	status := "accepted"
	summary := "x-automation-agent accepts the infrastructure automation handoff."
	result := map[string]any{
		"role":         s.role,
		"decision":     "accepted",
		"next_action":  "validate and produce an automation plan",
		"request_id":   req.RequestID,
		"target_agent": s.agentID,
	}

	if containsAny(text, []string{"logs", "metrics", "traces", "topology", "alert", "observability"}) {
		status = "needs_input"
		summary = "x-automation-agent needs observability evidence from x-observability-agent before change planning."
		result["decision"] = "consult"
		result["handoff_agent_id"] = "x-observability-agent"
	}
	if containsAny(text, []string{"incident", "root cause", "runbook", "pager", "outage"}) {
		status = "declined"
		summary = "x-automation-agent declines incident command and recommends xops-agent."
		result["decision"] = "handoff"
		result["handoff_agent_id"] = "xops-agent"
	}

	log.Printf("a2a negotiate request_id=%s from=%s to=%s status=%s", req.RequestID, req.FromAgentID, s.agentID, status)
	return Response{
		Status:       status,
		OwnerAgentID: s.agentID,
		Summary:      summary,
		Result:       result,
	}
}

func (s *Service) CreateTask(req Request) TaskRecord {
	taskID := newTaskID()
	resp := s.Negotiate(req)
	record := TaskRecord{
		TaskID:       taskID,
		RequestID:    req.RequestID,
		FromAgentID:  req.FromAgentID,
		ToAgentID:    fallback(req.ToAgentID, s.agentID),
		Intent:       req.Intent,
		Goal:         req.Goal,
		Status:       resp.Status,
		OwnerAgentID: s.agentID,
		Summary:      resp.Summary,
		Result:       resp.Result,
		CreatedAt:    time.Now().UTC(),
	}
	if record.Status == "accepted" {
		record.Status = "completed"
		record.Summary = "x-automation-agent completed the automation-side coordination and produced a plan."
		record.Result["deliverable"] = "automation_plan"
	}
	s.mu.Lock()
	s.tasks[taskID] = record
	s.mu.Unlock()
	log.Printf("a2a task request_id=%s task_id=%s from=%s to=%s status=%s", req.RequestID, taskID, req.FromAgentID, s.agentID, record.Status)
	return record
}

func (s *Service) GetTask(taskID string) (TaskRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.tasks[taskID]
	return record, ok
}

func decodeRequest(w http.ResponseWriter, r *http.Request) (Request, bool) {
	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return Request{}, false
	}
	req.RequestID = strings.TrimSpace(req.RequestID)
	if req.RequestID == "" {
		req.RequestID = taskSeed()
	}
	return req, true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func containsAny(text string, candidates []string) bool {
	for _, candidate := range candidates {
		if strings.Contains(text, candidate) {
			return true
		}
	}
	return false
}

func stringify(value any) string {
	if value == nil {
		return ""
	}
	blob, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(blob)
}

func fallback(value, def string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(def)
}

func newTaskID() string {
	return "a2a-" + taskSeed()
}

func taskSeed() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
	}
	return hex.EncodeToString(buf[:])
}
