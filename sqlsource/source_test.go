package sqlsource

import (
	"testing"

	coreauth "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-auth"
)

// I test girano sulle funzioni pure: la ricomposizione delle righe nello Snapshot e la lettura
// della lista di metodi. Non aggiungono a go-core-auth un dialect né un driver concreti, che
// restano dipendenze dell'applicazione — è la stessa disciplina dei test di go-core-sql.

func TestToSnapshot_RicomponeLeAssociazioni(t *testing.T) {
	snap := toSnapshot(
		[]contextRow{{ID: "NORD", Label: "Nord", HomeApp: "APP_HOME", Ord: 1}},
		[]appRow{{ID: "APP_ANAG", BasePath: "/anagrafica/", Ord: 2}},
		[]capabilityRow{
			{ID: "API_READ", Category: coreauth.CategoryAPI, APIPath: "/api/**", APIMethods: "GET, POST"},
			{ID: "UI_LIST", Category: coreauth.CategoryUI, Endpoint: "/persons", Menu: true, AppID: "APP_ANAG", Ord: 1},
		},
		[]capabilityGroupRow{{ID: "GRP", Description: "gruppo"}},
		[]capabilityGroupItemRow{
			{GroupID: "GRP", CapabilityID: "API_READ"},
			{GroupID: "GRP", CapabilityID: "UI_LIST"},
		},
		[]roleRow{
			{ID: "OP", ContextID: "NORD"},
			{ID: "ADMIN"},
		},
		[]roleGroupRow{{RoleID: "OP", GroupID: "GRP"}},
		[]roleCapRow{{RoleID: "ADMIN", CapabilityID: "API_READ"}},
	)

	if len(snap.CapabilityGroups) != 1 || len(snap.CapabilityGroups[0].Capabilities) != 2 {
		t.Fatalf("il gruppo deve riprendere le sue capability dalla tabella di associazione: %+v", snap.CapabilityGroups)
	}

	byID := map[string]coreauth.Role{}
	for _, r := range snap.Roles {
		byID[r.ID] = r
	}
	if g := byID["OP"].CapabilityGroups; len(g) != 1 || g[0] != "GRP" {
		t.Errorf("gruppi del ruolo OP = %v", g)
	}
	if c := byID["ADMIN"].Capabilities; len(c) != 1 || c[0] != "API_READ" {
		t.Errorf("capability dirette di ADMIN = %v", c)
	}
	if byID["ADMIN"].ContextID != "" {
		t.Error("un ruolo senza context_id deve restare context-agnostic")
	}

	if len(snap.Capabilities) != 2 || len(snap.Capabilities[0].APIMethods) != 2 {
		t.Errorf("capability = %+v", snap.Capabilities)
	}
	if snap.Contexts[0].HomeApp != "APP_HOME" || snap.Apps[0].BasePath != "/anagrafica/" {
		t.Errorf("contesti/app = %+v / %+v", snap.Contexts, snap.Apps)
	}
}

func TestToSnapshot_RuoloSenzaAssociazioni(t *testing.T) {
	snap := toSnapshot(nil, nil, nil, nil, nil, []roleRow{{ID: "VUOTO"}}, nil, nil)

	if len(snap.Roles) != 1 {
		t.Fatalf("ruoli = %+v", snap.Roles)
	}
	if snap.Roles[0].CapabilityGroups != nil || snap.Roles[0].Capabilities != nil {
		t.Error("un ruolo senza associazioni non deve ricevere liste finte")
	}
}

func TestSplitMethods(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{"GET", []string{"GET"}},
		{"GET,POST", []string{"GET", "POST"}},
		{" GET , POST ", []string{"GET", "POST"}},
		{"GET,,POST", []string{"GET", "POST"}},
	}
	for _, c := range cases {
		got := splitMethods(c.in)
		if len(got) != len(c.want) {
			t.Errorf("splitMethods(%q) = %v, atteso %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("splitMethods(%q) = %v, atteso %v", c.in, got, c.want)
				break
			}
		}
	}
}

// Una lista vuota di metodi significa "tutti i metodi": deve restare nil, non una slice con un
// elemento vuoto, che non corrisponderebbe ad alcun metodo e negherebbe tutto.
func TestSplitMethods_VuotoSignificaTutti(t *testing.T) {
	if got := splitMethods(""); got != nil {
		t.Errorf("= %v (len %d), attesa nil", got, len(got))
	}
}

func TestTables_OrdineDiCreazione(t *testing.T) {
	ts := tables()
	if len(ts) != 8 {
		t.Fatalf("attese 8 tabelle, ottenute %d", len(ts))
	}
	// Le tre tabelle di associazione stanno in coda: un vincolo di integrità referenziale
	// aggiunto in seguito non troverebbe altrimenti le tabelle referenziate.
	if _, ok := ts[5].(*capabilityGroupItemRow); !ok {
		t.Error("le tabelle di associazione devono essere create per ultime")
	}
}
