package apitest

import "testing"

func TestResponseSchemaDereferencesReusableResponse(t *testing.T) {
	contract := Load(t)
	schema := contract.responseSchema(t, "POST", "/api/v1/jobs/{job_id}/retry", 202, "application/json")
	if schema == nil {
		t.Fatal("reusable response $ref must resolve to its JSON schema")
	}
	resolved := contract.resolve(schema)
	required, _ := resolved["required"].([]any)
	if len(required) != 1 || required[0] != "job" {
		t.Fatalf("retry response schema required=%v, want [job]", required)
	}
}
