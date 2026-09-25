package yamlsource

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	coreauth "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-auth"
)

const aclYAML = `
contexts:
  - id: NORD
    label: Area Nord
    home-app: APP_HOME
    order: 1
apps:
  - id: APP_HOME
    base-path: /
    order: 1
  - id: APP_ANAG
    description: Anagrafica
    base-path: /anagrafica/
    order: 2
capabilities:
  - id: API_PERSONS
    category: api
    api:
      operation-id: getPersons
      path: /api/persons/**
      methods: [GET, POST]
  - id: UI_ANAG
    category: ui
    app-id: APP_ANAG
    ui:
      endpoint: /persons
      icon: people
      order: 1
      menu: true
capability-groups:
  - id: GRP_ANAG
    capabilities: [API_PERSONS, UI_ANAG]
roles:
  - id: OPERATORE
    context: NORD
    capability-groups: [GRP_ANAG]
  - id: ADMIN
    capabilities: [API_PERSONS]
`

func writeRules(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "auth-rules.yml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("scrittura del file di regole: %v", err)
	}
	return path
}

func TestLoad_FileCompleto(t *testing.T) {
	s := &source{path: writeRules(t, aclYAML)}

	snap, err := s.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(snap.Contexts) != 1 || snap.Contexts[0].HomeApp != "APP_HOME" {
		t.Errorf("contesti = %+v", snap.Contexts)
	}
	if len(snap.Apps) != 2 || snap.Apps[1].BasePath != "/anagrafica/" {
		t.Errorf("app = %+v", snap.Apps)
	}
	if len(snap.Capabilities) != 2 {
		t.Fatalf("capability = %+v", snap.Capabilities)
	}
	api := snap.Capabilities[0]
	if api.APIPath != "/api/persons/**" || len(api.APIMethods) != 2 || api.OperationID != "getPersons" {
		t.Errorf("capability api = %+v", api)
	}
	ui := snap.Capabilities[1]
	if ui.Endpoint != "/persons" || !ui.Menu || ui.AppID != "APP_ANAG" {
		t.Errorf("capability ui = %+v", ui)
	}
	if len(snap.Roles) != 2 || snap.Roles[1].ContextID != "" {
		t.Errorf("ruoli = %+v: il secondo è context-agnostic", snap.Roles)
	}
}

// Il file è riletto a ogni Load: modificarlo cambia i permessi al ricaricamento successivo, senza
// riavviare il processo.
func TestLoad_RileggeIlFileOgniVolta(t *testing.T) {
	path := writeRules(t, aclYAML)
	s := &source{path: path}

	if _, err := s.Load(context.Background()); err != nil {
		t.Fatalf("primo Load: %v", err)
	}
	if err := os.WriteFile(path, []byte("roles:\n  - id: SOLO_UNO\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	snap, err := s.Load(context.Background())
	if err != nil {
		t.Fatalf("secondo Load: %v", err)
	}
	if len(snap.Roles) != 1 || snap.Roles[0].ID != "SOLO_UNO" {
		t.Errorf("ruoli = %+v: il file modificato non è stato riletto", snap.Roles)
	}
}

func TestLoad_CatalogoAggiungeEIlFileVince(t *testing.T) {
	s := &source{
		path: writeRules(t, aclYAML),
		catalog: Catalog{
			Apps: []coreauth.AppDef{
				{ID: "APP_ANAG", Description: "dal catalogo"}, // ridichiarata dal file
				{ID: "APP_SCOPERTA", Description: "solo dal catalogo"},
			},
			Capabilities: []coreauth.Capability{
				{ID: "API_SCOPERTA", Category: coreauth.CategoryAPI, APIPath: "/api/x"},
			},
		},
	}

	snap, err := s.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	byID := map[string]coreauth.AppDef{}
	for _, a := range snap.Apps {
		byID[a.ID] = a
	}
	if byID["APP_ANAG"].Description != "Anagrafica" {
		t.Errorf("sullo stesso id deve vincere il file: %+v", byID["APP_ANAG"])
	}
	if byID["APP_SCOPERTA"].Description == "" {
		t.Error("l'entità presente solo nel catalogo deve sopravvivere")
	}
	if len(snap.Capabilities) != 3 {
		t.Errorf("capability = %d, attese 3 (2 dal file + 1 scoperta)", len(snap.Capabilities))
	}
}

func TestLoad_ErroriDiConfigurazione(t *testing.T) {
	if _, err := (&source{}).Load(context.Background()); err == nil {
		t.Error("senza rules-file configurato Load deve fallire")
	} else if err.Code != CodeNoRules {
		t.Errorf("codice = %q, atteso %q", err.Code, CodeNoRules)
	}

	if _, err := (&source{path: "/non/esiste.yml"}).Load(context.Background()); err == nil {
		t.Error("un file inesistente deve fallire")
	} else if err.Code != CodeReadRules {
		t.Errorf("codice = %q, atteso %q", err.Code, CodeReadRules)
	}

	s := &source{path: writeRules(t, "roles: [ non chiuso")}
	if _, err := s.Load(context.Background()); err == nil {
		t.Error("uno YAML malformato deve fallire")
	} else if err.Code != CodeParseRules {
		t.Errorf("codice = %q, atteso %q", err.Code, CodeParseRules)
	}
}

func TestMergeByID_ConservaLOrdine(t *testing.T) {
	type e struct{ id, v string }
	got := mergeByID(
		[]e{{"a", "base"}, {"b", "base"}, {"c", "base"}},
		[]e{{"b", "over"}, {"d", "nuovo"}},
		func(x e) string { return x.id },
	)
	want := []e{{"a", "base"}, {"b", "over"}, {"c", "base"}, {"d", "nuovo"}}
	if len(got) != len(want) {
		t.Fatalf("= %+v, atteso %+v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("= %+v, atteso %+v", got, want)
		}
	}
}
