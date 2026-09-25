package sqlsource

import (
	"slices"
	"testing"
)

func TestSchema_DescriveLeOttoTabelle(t *testing.T) {
	s := Schema()
	if len(s) != 8 {
		t.Fatalf("tabelle = %d, attese 8: %v", len(s), TableNames())
	}

	cols, ok := s["acl_capability"]
	if !ok {
		t.Fatalf("acl_capability assente: %v", TableNames())
	}
	for _, want := range []string{"id", "category", "operation_id", "api_path", "api_methods", "app_id", "menu", "ord"} {
		if !slices.Contains(cols, want) {
			t.Errorf("colonna %q assente da acl_capability: %v", want, cols)
		}
	}

	if got := s["acl_capability_group_item"]; !slices.Contains(got, "group_id") || !slices.Contains(got, "capability_id") {
		t.Errorf("acl_capability_group_item = %v", got)
	}
}

func TestTableNames_OrdineDiCreazione(t *testing.T) {
	names := TableNames()
	if names[0] != "acl_context" {
		t.Errorf("prima tabella = %q", names[0])
	}
	if names[len(names)-1] != "acl_role_cap" {
		t.Errorf("ultima tabella = %q: le associazioni vanno create per ultime", names[len(names)-1])
	}
}
