package mongosource

import (
	"testing"

	coreauth "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-auth"
)

func TestToSnapshot_SmistaPerEntityType(t *testing.T) {
	docs := []aclDoc{
		{ID: "NORD", ET: etContext, Label: "Nord", HomeApp: "HOME_NORD", Order: 1},
		{ID: "ANAG", ET: etApp, Path: "/anagrafica/", Description: "Anagrafica", Order: 2},
		{ID: "OP", ET: etRole, ContextID: "NORD", CapabilityGroups: []string{"G1"}, Capabilities: []string{"C1"}},
		{ID: "G1", ET: etCapabilityGroup, Capabilities: []string{"C1", "C2"}},
		{ID: "C1", ET: etCapability, Category: coreauth.CategoryAPI,
			API: &apiSpec{OperationID: "getPersons", Path: "/api/persons/**", Methods: []string{"GET"}}},
		{ID: "IGNOTO", ET: "QUALCOSALTRO"},
		{ID: "", ET: etApp}, // senza _id: scartato
	}

	snap := toSnapshot(docs)

	if len(snap.Contexts) != 1 || snap.Contexts[0].HomeApp != "HOME_NORD" {
		t.Errorf("contesti = %+v", snap.Contexts)
	}
	if len(snap.Apps) != 1 || snap.Apps[0].BasePath != "/anagrafica/" {
		t.Errorf("app = %+v: `path` del documento deve diventare BasePath", snap.Apps)
	}
	if len(snap.Roles) != 1 || snap.Roles[0].ContextID != "NORD" {
		t.Errorf("ruoli = %+v: `_cid` deve diventare ContextID", snap.Roles)
	}
	if len(snap.CapabilityGroups) != 1 || len(snap.CapabilityGroups[0].Capabilities) != 2 {
		t.Errorf("gruppi = %+v", snap.CapabilityGroups)
	}
	if len(snap.Capabilities) != 1 {
		t.Fatalf("capability = %+v", snap.Capabilities)
	}
	if len(snap.Roles[0].Capabilities) != 1 {
		t.Error("le capability dirette del ruolo non vanno risolte qui: le espande l'engine")
	}
}

func TestToCapability_SottodocumentiPerCategoria(t *testing.T) {
	api := toCapability(aclDoc{
		ID: "API", Category: coreauth.CategoryAPI,
		API: &apiSpec{OperationID: "op", Path: "/api/**", Methods: []string{"GET", "POST"}},
	})
	if api.APIPath != "/api/**" || api.OperationID != "op" || len(api.APIMethods) != 2 {
		t.Errorf("capability api = %+v", api)
	}

	ui := toCapability(aclDoc{
		ID: "UI", Category: coreauth.CategoryUI, AppID: "ANAG", Order: 1, Icon: "top",
		UI: &uiSpec{Endpoint: "/persons", Icon: "people", Order: 5, Menu: true},
	})
	if ui.Endpoint != "/persons" || !ui.Menu {
		t.Errorf("capability ui = %+v", ui)
	}
	if ui.Icon != "people" || ui.Order != 5 {
		t.Errorf("icona e ordine del sottodocumento ui devono vincere su quelli di primo livello: %+v", ui)
	}
}

func TestToCapability_SenzaSottodocumentiNonEsplode(t *testing.T) {
	c := toCapability(aclDoc{ID: "ACT", Category: coreauth.CategoryActionAPI, Description: "Export"})
	if c.ID != "ACT" || c.APIPath != "" || c.Endpoint != "" {
		t.Errorf("capability = %+v", c)
	}
}
