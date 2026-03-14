package openclaw

import "testing"

func TestRegistrationCreateName(t *testing.T) {
	if got := registrationCreateName("x-automation-agent", "XCloudFlowAutomation"); got != "x-automation-agent" {
		t.Fatalf("expected fallback to agent id, got %q", got)
	}
	if got := registrationCreateName("x-automation-agent", "x-automation-agent"); got != "x-automation-agent" {
		t.Fatalf("expected matching name to be preserved, got %q", got)
	}
}

func TestNormalizeAgentID(t *testing.T) {
	if got := normalizeAgentID("X Automation/Agent"); got != "x-automation-agent" {
		t.Fatalf("unexpected normalized id: %q", got)
	}
}
