package dbfailure

import (
	"database/sql"
	"testing"
)

func ns(s string) sql.NullString { return sql.NullString{String: s, Valid: true} }

func TestValidReason(t *testing.T) {
	for reason, want := range map[string]bool{
		"": false, "A": true, "AB_C1": true, "a": false, "1A": false, "A__B": false, "A_": false, "A-B": false,
	} {
		if got := ValidReason(reason); got != want {
			t.Errorf("ValidReason(%q) = %v, want %v", reason, got, want)
		}
	}
}

func TestColumns(t *testing.T) {
	r, p, d, err := Columns(nil)
	if err != nil || r.Valid || p.Valid || d.Valid {
		t.Fatalf("nil failure: %v %v %v %v", r, p, d, err)
	}
	r, p, d, err = Columns(&Failure{Reason: "BOOM"})
	if err != nil || r != ns("BOOM") || p != ns("{}") || d.Valid {
		t.Fatalf("nil params: %v %v %v %v", r, p, d, err)
	}
	r, p, d, err = Columns(&Failure{Reason: "BOOM", Params: map[string]string{"k": "v"}, TechnicalDetail: "x"})
	if err != nil || r != ns("BOOM") || p != ns(`{"k":"v"}`) || d != ns("x") {
		t.Fatalf("full: %v %v %v %v", r, p, d, err)
	}
	if _, _, _, err = Columns(&Failure{Reason: "bad"}); err == nil || err.Error() != `invalid reason "bad"` {
		t.Fatalf("bad reason: %v", err)
	}
}

func TestFromRow(t *testing.T) {
	none := sql.NullString{}
	if f, err := FromRow(none, none, none, none, "UNKNOWN"); f != nil || err != nil {
		t.Fatalf("empty: %v %v", f, err)
	}
	f, err := FromRow(none, none, none, ns(" legacy "), "UNKNOWN")
	if err != nil || f.Reason != "UNKNOWN" || f.TechnicalDetail != "legacy" || f.Params != nil {
		t.Fatalf("legacy: %+v %v", f, err)
	}
	if _, err = FromRow(none, ns("{}"), none, none, "UNKNOWN"); err == nil || err.Error() != "failure params/detail present without reason" {
		t.Fatalf("orphan params: %v", err)
	}
	if _, err = FromRow(ns("bad"), ns("{}"), none, none, "U"); err == nil || err.Error() != `invalid reason "bad"` {
		t.Fatalf("bad reason: %v", err)
	}
	if _, err = FromRow(ns("BOOM"), ns("[]"), none, none, "U"); err == nil || err.Error() != "failure params must be a JSON object" {
		t.Fatalf("non-object: %v", err)
	}
	if _, err = FromRow(ns("BOOM"), none, none, none, "U"); err == nil || err.Error() != "failure params must be a JSON object" {
		t.Fatalf("null params: %v", err)
	}
	if _, err = FromRow(ns("BOOM"), ns(`{"a":1}`), none, none, "U"); err == nil {
		t.Fatal("non-string value should fail decode")
	}
	f, err = FromRow(ns("BOOM"), ns("{}"), ns("d"), none, "U")
	if err != nil || f.Params != nil || f.Reason != "BOOM" || f.TechnicalDetail != "d" {
		t.Fatalf("empty params: %+v %v", f, err)
	}
	f, err = FromRow(ns("BOOM"), ns(`{"k":"v"}`), none, none, "U")
	if err != nil || f.Params["k"] != "v" {
		t.Fatalf("params: %+v %v", f, err)
	}
}
