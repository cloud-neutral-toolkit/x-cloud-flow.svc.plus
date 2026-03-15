package store

import "time"

type Run struct {
	RunID      string
	Stack      string
	Env        string
	Phase      string
	Status     string
	Actor      string
	ConfigRef  string
	StartedAt  time.Time
	FinishedAt *time.Time
	InputsJSON []byte
	PlanJSON   []byte
	ResultJSON []byte
}

type MCPServer struct {
	ServerID  string
	Name      string
	BaseURL   string
	Kind      string
	AuthType  string
	Audience  string
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type SkillSource struct {
	SourceID string
	Name     string
	Type     string
	URI      string
	Ref      string
	BasePath string
	Enabled  bool
}

type StateObject struct {
	TenantID      string
	ObjectKey     string
	Version       int64
	Tool          string
	Project       string
	Env           string
	ResourceScope string
	ContentJSON   []byte
	ContentBytes  []byte
	ETag          string
	Actor         string
	CreatedAt     time.Time
}

type StateLock struct {
	TenantID  string
	ObjectKey string
	LockID    string
	Owner     string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type ChangeSet struct {
	TenantID    string
	ChangeSetID string
	Project     string
	Env         string
	Phase       string
	Status      string
	Actor       string
	Summary     string
	InputsJSON  []byte
	PlanJSON    []byte
	ResultJSON  []byte
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ResourceRecord struct {
	TenantID          string
	ResourceUID       string
	Project           string
	ResourceType      string
	Cloud             string
	Region            string
	Env               string
	Engine            string
	Provider          string
	ExternalID        string
	Name              string
	LabelsJSON        []byte
	DesiredStateJSON  []byte
	ObservedStateJSON []byte
	DriftStatus       string
	StateObjectKey    string
	LastChangeSetID   string
	LastSeenAt        *time.Time
	UpdatedAt         time.Time
}

type ResourceEvent struct {
	TenantID    string
	EventID     string
	ResourceUID string
	ChangeSetID string
	EventType   string
	DiffJSON    []byte
	Message     string
	CreatedAt   time.Time
}
