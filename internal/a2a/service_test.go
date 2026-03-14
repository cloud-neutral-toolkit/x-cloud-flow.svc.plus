package a2a

import "testing"

func TestNegotiateDeclinesIncidentGoal(t *testing.T) {
	svc := NewService("x-automation-agent", "automation")
	resp := svc.Negotiate(Request{
		FromAgentID: "xops-agent",
		RequestID:   "req-1",
		Intent:      "coordinate",
		Goal:        "root cause analysis for outage",
	})
	if resp.Status != "declined" {
		t.Fatalf("expected declined, got %s", resp.Status)
	}
	if got := resp.Result["handoff_agent_id"]; got != "xops-agent" {
		t.Fatalf("expected ops handoff, got %#v", got)
	}
}

func TestCreateTaskCompletesAutomationPlan(t *testing.T) {
	svc := NewService("x-automation-agent", "automation")
	task := svc.CreateTask(Request{
		FromAgentID: "xops-agent",
		RequestID:   "req-2",
		Intent:      "plan",
		Goal:        "generate terraform and dns remediation plan",
	})
	if task.Status != "completed" {
		t.Fatalf("expected completed, got %s", task.Status)
	}
}
