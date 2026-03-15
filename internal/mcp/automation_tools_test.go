package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestTerraformRecordsFromStateFile(t *testing.T) {
	dir := t.TempDir()
	state := `{
	  "resources": [
	    {
	      "mode": "managed",
	      "type": "aws_instance",
	      "name": "web",
	      "provider": "provider[\"registry.terraform.io/hashicorp/aws\"]",
	      "instances": [
	        {
	          "attributes": {
	            "id": "i-123",
	            "name": "web-1",
	            "tags": {"Name": "web-1"}
	          }
	        }
	      ]
	    }
	  ]
	}`
	path := filepath.Join(dir, "terraform.tfstate")
	if err := os.WriteFile(path, []byte(state), 0o644); err != nil {
		t.Fatalf("write tfstate: %v", err)
	}

	records, err := terraformRecordsFromStateFile(path, terraformToolInput{
		Module:      "network",
		WorkingDir:  dir,
		ChangeSetID: "cs-1",
		TenantID:    "tenant-a",
	}, "apply")
	if err != nil {
		t.Fatalf("terraformRecordsFromStateFile: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].ExternalID != "i-123" {
		t.Fatalf("unexpected external id: %s", records[0].ExternalID)
	}
	if records[0].Cloud != "aws" {
		t.Fatalf("unexpected cloud: %s", records[0].Cloud)
	}
	if records[0].TenantID != "tenant-a" {
		t.Fatalf("unexpected tenant id: %s", records[0].TenantID)
	}
}

func TestSyntheticAnsiblePlaybookRecord(t *testing.T) {
	runRes := map[string]any{"mode": "apply"}
	rec, evt := syntheticAnsiblePlaybookRecord(ansibleToolInput{
		Playbook:    "site.yml",
		Inventory:   "inventory.ini",
		ChangeSetID: "cs-2",
		TenantID:    "tenant-a",
	}, runRes, false)
	if rec.ResourceType != "ansible_playbook" {
		t.Fatalf("unexpected resource type: %s", rec.ResourceType)
	}
	if evt.ChangeSetID != "cs-2" {
		t.Fatalf("unexpected change set id: %s", evt.ChangeSetID)
	}
	if rec.TenantID != "tenant-a" {
		t.Fatalf("unexpected tenant id: %s", rec.TenantID)
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.ObservedStateJSON, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["playbook"] != "site.yml" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}
