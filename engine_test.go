package coreauth

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	core "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
)

// fakeSource restituisce lo snapshot che il test gli mette in mano, o un errore.
type fakeSource struct {
	snap  atomic.Pointer[Snapshot]
	fail  atomic.Bool
	calls atomic.Int32
}

func (f *fakeSource) Load(context.Context) (*Snapshot, *core.ApplicationError) {
	f.calls.Add(1)
	if f.fail.Load() {
		return nil, core.TechnicalError().WithAmbit(Ambit).WithCode(CodeSourceLoad).
			WithCause(errors.New("backend irraggiungibile"))
	}
	return f.snap.Load(), nil
}

// newTestLUT costruisce un engine già caricato con snap, con un refresh lungo: i test che vogliono
// un ricaricamento lo chiedono esplicitamente, così nessuna asserzione dipende da una goroutine.
func newTestLUT(t *testing.T, snap *Snapshot) (*lut, *fakeSource) {
	t.Helper()
	src := &fakeSource{}
	src.snap.Store(snap)
	l := newLUT(src, time.Hour, time.Second)
	if err := l.load(context.Background()); err != nil {
		t.Fatalf("caricamento iniziale fallito: %v", err)
	}
	return l, src
}

// aclCompleto è l'ACL di riferimento dei test: due contesti con home dedicate diverse, un'app
// applicativa, un ruolo per contesto e un ruolo agnostic.
func aclCompleto() *Snapshot {
	return &Snapshot{
		Contexts: []ContextDef{
			{ID: "NORD", Label: "Nord", HomeApp: "HOME_NORD", Order: 1},
			{ID: "SUD", Label: "Sud", HomeApp: "HOME_SUD", Order: 2},
		},
		Apps: []AppDef{
			{ID: "HOME_NORD", BasePath: "/", Order: 1},
			{ID: "HOME_SUD", BasePath: "/", Order: 2},
			{ID: "ANAGRAFICA", BasePath: "/anagrafica/", Order: 3},
		},
		Capabilities: []Capability{
			{ID: "UI_ANAG", Category: CategoryUI, AppID: "ANAGRAFICA", Endpoint: "/persons", Menu: true, Order: 2},
			{ID: "UI_GLOBAL", Category: CategoryUI, Endpoint: "/about", Menu: true, Order: 1},
			{ID: "ACT_EDIT", Category: CategoryActionUI, AppID: "ANAGRAFICA"},
			{ID: "ACT_GLOBAL", Category: CategoryActionUI},
			{ID: "API_PERSONS", Category: CategoryAPI, APIPath: "/api/persons/**", APIMethods: []string{"GET"}},
			{ID: "API_ANY", Category: CategoryAPI, APIPath: "/api/health"},
			{ID: "ACTAPI_EXPORT", Category: CategoryActionAPI},
		},
		CapabilityGroups: []CapabilityGroup{
			{ID: "GRP_ANAG", Capabilities: []string{"UI_ANAG", "ACT_EDIT", "API_PERSONS"}},
		},
		Roles: []Role{
			{ID: "OP_NORD", ContextID: "NORD", CapabilityGroups: []string{"GRP_ANAG"}},
			{ID: "OP_SUD", ContextID: "SUD", CapabilityGroups: []string{"GRP_ANAG"}},
			{ID: "ADMIN", Capabilities: []string{"UI_GLOBAL", "ACT_GLOBAL", "API_ANY", "ACTAPI_EXPORT"}},
		},
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// --- risoluzione dell'ACL ---

func TestNewView_EspandeIGruppiEDeduplica(t *testing.T) {
	snap := aclCompleto()
	// La capability è già nel gruppo: dichiararla anche diretta non deve duplicarla.
	snap.Roles[0].Capabilities = []string{"UI_ANAG", "ACT_GLOBAL"}

	v, dangling := newView(snap)
	if len(dangling) != 0 {
		t.Fatalf("riferimenti pendenti inattesi: %v", dangling)
	}
	caps := v.roleCaps["OP_NORD"]
	if len(caps) != 4 {
		t.Fatalf("attese 4 capability (3 dal gruppo + 1 diretta nuova), ottenute %d: %v", len(caps), caps)
	}
	seen := map[string]int{}
	for _, c := range caps {
		seen[c.ID]++
	}
	if seen["UI_ANAG"] != 1 {
		t.Errorf("UI_ANAG presente %d volte, attesa 1 (gruppo e diretta sono la stessa)", seen["UI_ANAG"])
	}
}

func TestNewView_RiferimentiPendentiNonInvalidanoLACL(t *testing.T) {
	snap := aclCompleto()
	snap.Roles = append(snap.Roles, Role{
		ID:               "ROTTO",
		CapabilityGroups: []string{"GRP_INESISTENTE"},
		Capabilities:     []string{"CAP_INESISTENTE"},
	})

	v, dangling := newView(snap)
	if len(dangling) != 2 {
		t.Fatalf("attesi 2 riferimenti pendenti, ottenuti %d: %v", len(dangling), dangling)
	}
	if len(v.roleCaps["OP_NORD"]) == 0 {
		t.Error("un ruolo rotto non deve invalidare gli altri")
	}
}

// --- S4: il case dei contesti è normalizzato una volta sola ---

func TestContestoNormalizzato_HomeSiTrovaInOgniCase(t *testing.T) {
	l, _ := newTestLUT(t, aclCompleto())

	for _, cid := range []string{"NORD", "nord", "Nord", " nord "} {
		if got := l.HomeAppForContext(cid); got != "HOME_NORD" {
			t.Errorf("HomeAppForContext(%q) = %q, attesa HOME_NORD", cid, got)
		}
	}
	if got := l.FilterRolesByContext([]string{"OP_NORD"}, "nord"); len(got) != 1 {
		t.Errorf("FilterRolesByContext con contesto minuscolo = %v, atteso [OP_NORD]", got)
	}
}

// --- FilterRolesByContext ---

func TestFilterRolesByContext(t *testing.T) {
	l, _ := newTestLUT(t, aclCompleto())
	roles := []string{"OP_NORD", "OP_SUD", "ADMIN"}

	got := l.FilterRolesByContext(roles, "NORD")
	if len(got) != 2 || !contains(got, "OP_NORD") || !contains(got, "ADMIN") {
		t.Errorf("= %v, atteso [OP_NORD ADMIN]: il ruolo dell'altro contesto esce, l'agnostic resta", got)
	}
	if got := l.FilterRolesByContext(roles, ""); len(got) != 3 {
		t.Errorf("con contesto vuoto i ruoli restano invariati, ottenuto %v", got)
	}
	if got := l.FilterRolesByContext([]string{"OP_SUD"}, "NORD"); len(got) != 0 {
		t.Errorf("= %v, atteso vuoto: nessun ruolo valido su quel contesto", got)
	}
}

// --- S2: un ruolo agnostic vede tutti i contesti ---

func TestGetContexts_RuoloAgnosticVedeTuttiIContesti(t *testing.T) {
	l, _ := newTestLUT(t, aclCompleto())

	got := l.GetContexts([]string{"ADMIN"})
	if len(got) != 2 {
		t.Fatalf("un ruolo agnostic deve vedere tutti i contesti, ottenuti %d: %v", len(got), got)
	}
	if got[0].ID != "NORD" || got[1].ID != "SUD" {
		t.Errorf("ordine non deterministico o errato: %v/%v", got[0].ID, got[1].ID)
	}
}

func TestGetContexts_RuoloContestualeVedeSoloIlSuo(t *testing.T) {
	l, _ := newTestLUT(t, aclCompleto())

	got := l.GetContexts([]string{"OP_NORD"})
	if len(got) != 1 || got[0].ID != "NORD" {
		t.Fatalf("= %v, atteso il solo NORD", got)
	}
	if got[0].Label != "Nord" {
		t.Errorf("label = %q, attesa Nord", got[0].Label)
	}
}

// --- S1: la home la designa il contesto, non il path ---

func TestGetApps_HomeDesignataDalContesto(t *testing.T) {
	l, _ := newTestLUT(t, aclCompleto())

	apps := l.GetApps([]string{"OP_NORD"}, "NORD")
	var home, anag *App
	for _, a := range apps {
		switch a.ID {
		case "HOME_NORD":
			home = a
		case "ANAGRAFICA":
			anag = a
		case "HOME_SUD":
			t.Error("la home dell'altro contesto non deve comparire")
		}
	}
	if home == nil {
		t.Fatal("la home designata dal contesto deve comparire")
	}
	if home.Path != "/nord/" {
		t.Errorf("path della home = %q, atteso /nord/", home.Path)
	}
	if anag == nil {
		t.Fatal("l'app raggiungibile dalle capability ui deve comparire")
	}
	if anag.Path != "/nord/anagrafica/" {
		t.Errorf("path dell'app = %q, atteso /nord/anagrafica/", anag.Path)
	}
	if anag.ContextID != "NORD" {
		t.Errorf("context id = %q, atteso NORD", anag.ContextID)
	}
}

func TestGetApps_SenzaDesignazioneRicadeSulBasePath(t *testing.T) {
	snap := aclCompleto()
	snap.Contexts = []ContextDef{{ID: "NORD", Label: "Nord"}} // nessuna home designata
	snap.Apps = []AppDef{
		{ID: "HOME", BasePath: "/", Order: 1},
		{ID: "ANAGRAFICA", BasePath: "/anagrafica/", Order: 2},
	}
	l, _ := newTestLUT(t, snap)

	if got := l.HomeAppForContext("NORD"); got != "HOME" {
		t.Errorf("HomeAppForContext = %q, attesa HOME dal fallback su BasePath /", got)
	}
	apps := l.GetApps([]string{"OP_NORD"}, "NORD")
	found := false
	for _, a := range apps {
		if a.ID == "HOME" && a.Path == "/nord/" {
			found = true
		}
	}
	if !found {
		t.Errorf("la home di fallback non è stata esposta: %v", apps)
	}
}

func TestGetApps_ContestiDichiaratiMaNonScelto_SoloLaHome(t *testing.T) {
	l, _ := newTestLUT(t, aclCompleto())

	apps := l.GetApps([]string{"OP_NORD"}, "")
	for _, a := range apps {
		if a.ID == "ANAGRAFICA" {
			t.Errorf("senza contesto selezionato le app applicative non vanno esposte: %v", apps)
		}
		if a.ContextID != "" {
			t.Errorf("senza contesto il path non va prefissato: %+v", a)
		}
	}
}

func TestGetApps_AclSenzaContesti_AppEsposteSubito(t *testing.T) {
	snap := aclCompleto()
	snap.Contexts = nil
	snap.Roles = []Role{{ID: "OP", CapabilityGroups: []string{"GRP_ANAG"}}}
	l, _ := newTestLUT(t, snap)

	apps := l.GetApps([]string{"OP"}, "")
	found := false
	for _, a := range apps {
		if a.ID == "ANAGRAFICA" {
			found = true
			if a.Path != "/anagrafica/" {
				t.Errorf("path = %q, atteso quello dichiarato senza prefisso", a.Path)
			}
		}
	}
	if !found {
		t.Errorf("un ACL senza contesti non ha la fase di selezione: le app vanno esposte subito, ottenute %v", apps)
	}
}

func TestGetApps_AppNonRaggiungibileNonCompare(t *testing.T) {
	l, _ := newTestLUT(t, aclCompleto())

	// ADMIN ha solo capability globali: nessuna ui legata ad ANAGRAFICA.
	for _, a := range l.GetApps([]string{"ADMIN"}, "NORD") {
		if a.ID == "ANAGRAFICA" {
			t.Errorf("app esposta senza alcuna capability ui che la nomini: %v", a)
		}
	}
}

// --- S3: la capability senza app-id vale per ogni app, nei menu come nelle azioni ---

func TestCapabilityGlobaleValePerOgniApp(t *testing.T) {
	l, _ := newTestLUT(t, aclCompleto())

	paths := l.GetPaths([]string{"ADMIN"}, "ANAGRAFICA")
	if len(paths) != 1 || paths[0].ID != "UI_GLOBAL" {
		t.Errorf("GetPaths = %v, attesa la voce globale anche interrogando un'app specifica", paths)
	}
	caps := l.GetCapabilities([]string{"ADMIN"}, "ANAGRAFICA")
	if len(caps) != 1 || caps[0] != "ACT_GLOBAL" {
		t.Errorf("GetCapabilities = %v, attesa l'azione globale", caps)
	}
}

func TestGetPaths_FiltraPerApp(t *testing.T) {
	l, _ := newTestLUT(t, aclCompleto())

	paths := l.GetPaths([]string{"OP_NORD"}, "ALTRA_APP")
	for _, p := range paths {
		if p.ID == "UI_ANAG" {
			t.Errorf("voce di un'altra app esposta: %v", paths)
		}
	}
	paths = l.GetPaths([]string{"OP_NORD"}, "ANAGRAFICA")
	if len(paths) != 1 || paths[0].ID != "UI_ANAG" {
		t.Errorf("= %v, attesa la sola UI_ANAG", paths)
	}
}

// --- S6: HasCapability è sul contratto e guarda solo le action_api ---

func TestHasCapability_SoloActionApi(t *testing.T) {
	var a Authorizer
	l, _ := newTestLUT(t, aclCompleto())
	a = l // il contratto deve bastare: nessun type assert sul concreto

	if !a.HasCapability([]string{"ADMIN"}, "ACTAPI_EXPORT") {
		t.Error("l'azione action_api del ruolo non è stata riconosciuta")
	}
	if a.HasCapability([]string{"ADMIN"}, "ACT_GLOBAL") {
		t.Error("una action_ui non deve rispondere a HasCapability")
	}
	if a.HasCapability([]string{"OP_NORD"}, "ACTAPI_EXPORT") {
		t.Error("capability concessa a un ruolo che non la possiede")
	}
}

// --- MatchRequest ---

func TestMatchRequest(t *testing.T) {
	l, _ := newTestLUT(t, aclCompleto())
	roles := []string{"OP_NORD"}

	cases := []struct {
		path, method string
		want         bool
	}{
		{"/api/persons", "GET", true},
		{"/api/persons/123", "GET", true},
		{"/api/persons/123/orders", "GET", true},
		{"/api/persons/123", "POST", false}, // metodo non dichiarato
		{"/api/orders", "GET", false},       // path fuori dal glob
	}
	for _, c := range cases {
		if got := l.MatchRequest(roles, c.path, c.method); got != c.want {
			t.Errorf("MatchRequest(%s %s) = %v, atteso %v", c.method, c.path, got, c.want)
		}
	}

	// Capability senza metodi: vale per tutti.
	if !l.MatchRequest([]string{"ADMIN"}, "/api/health", "DELETE") {
		t.Error("una capability senza metodi dichiarati deve valere per ogni metodo")
	}
}

func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"/api/persons", "/api/persons", true},
		{"/api/**", "/api", true},
		{"/api/**", "/api/persons/1", true},
		{"/api/**", "/apiary", false},
		{"/api/persons/*", "/api/persons/1", true},
		{"/api/persons/*", "/api/persons/1/orders", false},
		{"/api/persons/:id", "/api/persons/1", true},
		{"/api/persons/{id}", "/api/persons/1", true},
		{"/api/persons/:id/orders", "/api/persons/1/orders", true},
	}
	for _, c := range cases {
		if got := matchGlob(c.pattern, c.path); got != c.want {
			t.Errorf("matchGlob(%q, %q) = %v, atteso %v", c.pattern, c.path, got, c.want)
		}
	}
}

// --- trappole del ricaricamento ---

func TestRefresh_RuoloRevocatoSparisce(t *testing.T) {
	l, src := newTestLUT(t, aclCompleto())
	if !l.MatchRequest([]string{"OP_NORD"}, "/api/persons/1", "GET") {
		t.Fatal("presupposto del test non valido: il ruolo era autorizzato")
	}

	revocato := aclCompleto()
	revocato.Roles = revocato.Roles[1:] // via OP_NORD
	src.snap.Store(revocato)
	if err := l.load(context.Background()); err != nil {
		t.Fatalf("ricaricamento fallito: %v", err)
	}

	if l.MatchRequest([]string{"OP_NORD"}, "/api/persons/1", "GET") {
		t.Error("un ruolo tolto dall'ACL resta autorizzato: la view non è stata sostituita in blocco")
	}
}

func TestRefresh_ContestoRimossoSparisce(t *testing.T) {
	l, src := newTestLUT(t, aclCompleto())

	ridotto := aclCompleto()
	ridotto.Contexts = ridotto.Contexts[:1] // via SUD
	src.snap.Store(ridotto)
	if err := l.load(context.Background()); err != nil {
		t.Fatalf("ricaricamento fallito: %v", err)
	}

	if ids := l.AllContextIDs(); len(ids) != 1 || ids[0] != "NORD" {
		t.Errorf("AllContextIDs = %v, atteso il solo NORD dopo la rimozione", ids)
	}
	if got := l.HomeAppForContext("SUD"); got == "HOME_SUD" {
		t.Error("la home di un contesto rimosso è ancora risolta")
	}
}

func TestRefresh_FallimentoConservaLoSnapshotPrecedente(t *testing.T) {
	l, src := newTestLUT(t, aclCompleto())

	src.fail.Store(true)
	if err := l.load(context.Background()); err == nil {
		t.Fatal("atteso errore dalla sorgente")
	}

	if !l.MatchRequest([]string{"OP_NORD"}, "/api/persons/1", "GET") {
		t.Error("un caricamento fallito ha svuotato la view: deve restare quella precedente")
	}
}

func TestLoad_SnapshotNulloSenzaErroreÈUnErrore(t *testing.T) {
	src := &fakeSource{} // snap nil, fail false
	l := newLUT(src, time.Hour, time.Second)

	err := l.load(context.Background())
	if err == nil {
		t.Fatal("uno snapshot nullo senza errore deve essere segnalato, non installato come ACL vuoto")
	}
	if err.Code != CodeSourceLoad {
		t.Errorf("codice = %q, atteso %q", err.Code, CodeSourceLoad)
	}
}

func TestViewVuota_NegaTutto(t *testing.T) {
	src := &fakeSource{}
	src.fail.Store(true)
	l := newLUT(src, time.Hour, time.Second)

	if l.MatchRequest([]string{"ADMIN"}, "/api/health", "GET") {
		t.Error("prima di un caricamento riuscito non va concesso nulla")
	}
	if len(l.AllContextIDs()) != 0 {
		t.Error("la view vuota non deve esporre contesti")
	}
	if l.HasCapability([]string{"ADMIN"}, "ACTAPI_EXPORT") {
		t.Error("la view vuota non deve concedere capability")
	}
}

func TestOrdinamentoDeterministico(t *testing.T) {
	l, _ := newTestLUT(t, aclCompleto())

	for i := range 20 {
		if got := l.GetServerCapabilities([]string{"OP_NORD", "ADMIN"}); len(got) != 2 ||
			got[0] != "API_ANY" || got[1] != "API_PERSONS" {
			t.Fatalf("iterazione %d: ordine non deterministico: %v", i, got)
		}
	}
}

func TestConfigWithDefaults(t *testing.T) {
	c := (&Config{}).WithDefaults()
	if c.Refresh != DefaultRefresh || c.LoadTimeout != DefaultLoadTimeout {
		t.Errorf("default di refresh/timeout non applicati: %+v", c)
	}
	if c.Mongo.Collection != DefaultCollection {
		t.Errorf("default della sorgente mongo non applicato: %+v", c)
	}
	if c.FailFastOnBoot {
		t.Error("fail-fast-on-boot deve essere spento di default: si parte negando, non fallendo")
	}

	// Idempotenza: un secondo giro non deve cambiare nulla.
	before := *c
	if after := c.WithDefaults(); *after != before {
		t.Error("WithDefaults non è idempotente")
	}
}
