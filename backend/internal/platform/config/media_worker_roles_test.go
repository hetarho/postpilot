package config

import "testing"

func TestMediaWorkerRolesAreDeploymentBound(t *testing.T) {
	creds := map[string]string{"native": "credential", "verify": "other"}
	roles, e := ParseMediaWorkerRoles("", creds)
	if e != nil || roles["verify"] != "native" {
		t.Fatalf("legacy=%v %v", roles, e)
	}
	roles, e = ParseMediaWorkerRoles(`{"verify":"analysis-verification"}`, creds)
	if e != nil || roles["verify"] != "analysis-verification" || roles["native"] != "native" {
		t.Fatalf("roles=%v %v", roles, e)
	}
	for _, raw := range []string{`{"unknown":"analysis-verification"}`, `{"verify":"gpu"}`, `{"verify":"native","verify":"analysis-verification"}`, `{"verify":"analysis-verification"} {}`} {
		if _, e := ParseMediaWorkerRoles(raw, creds); e == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	t.Setenv("MEDIA_WORKER_ROLE", "bad")
	if _, e := LoadWorkerRuntime(); e == nil {
		t.Fatal("unknown worker role accepted")
	}
}
