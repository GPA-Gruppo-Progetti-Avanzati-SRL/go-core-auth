package apiauth

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	coreauth "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-auth"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
)

// stubAuthorizer risponde ciò che il test gli dice: i test del middleware riguardano il percorso
// HTTP, non le decisioni dell'engine, che hanno i loro.
type stubAuthorizer struct {
	allow        bool
	contextRoles []string
}

func (s *stubAuthorizer) FilterRolesByContext(roles []string, _ string) []string {
	if s.contextRoles != nil {
		return s.contextRoles
	}
	return roles
}
func (s *stubAuthorizer) MatchRequest([]string, string, string) bool { return s.allow }
func (s *stubAuthorizer) GetContexts([]string) []*coreauth.Context   { return nil }
func (s *stubAuthorizer) GetApps([]string, string) []*coreauth.App   { return nil }
func (s *stubAuthorizer) GetPaths([]string, string) []*coreauth.Path { return nil }
func (s *stubAuthorizer) GetCapabilities([]string, string) []string  { return []string{"CAP"} }
func (s *stubAuthorizer) GetServerCapabilities([]string) []string    { return nil }
func (s *stubAuthorizer) HasCapability([]string, string) bool        { return false }
func (s *stubAuthorizer) AllContextIDs() []string                    { return nil }
func (s *stubAuthorizer) HomeAppForContext(string) string            { return "" }

type probeOutput struct {
	Body probe
}

type probe struct {
	User      string
	Roles     []string
	ContextID string
}

// newTestAPI monta il middleware e una rotta che riporta ciò che ha trovato nel context.
func newTestAPI(t *testing.T, m *Middleware) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t, huma.DefaultConfig("test", "1.0.0"))
	m.Register(api, nil)
	huma.Get(api, "/probe", func(ctx context.Context, _ *struct{}) (*probeOutput, error) {
		return &probeOutput{Body: probe{
			User: UserFrom(ctx), Roles: RolesFrom(ctx), ContextID: ContextIDFrom(ctx),
		}}, nil
	})
	return api
}

func cfg(enabled bool, guest ...string) *coreauth.MiddlewareConfig {
	return (&coreauth.MiddlewareConfig{Enabled: enabled, GuestPaths: guest}).WithDefaults()
}

func TestMiddleware_SenzaRuoliNega(t *testing.T) {
	api := newTestAPI(t, New(&stubAuthorizer{allow: true}, cfg(true)))

	resp := api.Get("/probe")
	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, atteso 403", resp.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("il corpo del 403 non è JSON: %s", resp.Body.String())
	}
	if body["ambit"] != coreauth.Ambit || body["code"] != coreauth.CodeForbiddenRole {
		t.Errorf("corpo = %v: il rifiuto deve avere la forma ambit/code/message", body)
	}
}

func TestMiddleware_RuoloNonAbilitatoNega(t *testing.T) {
	api := newTestAPI(t, New(&stubAuthorizer{allow: false}, cfg(true)))

	resp := api.Get("/probe", "X-Roles: OSPITE")
	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, atteso 403", resp.Code)
	}
}

func TestMiddleware_RuoloAbilitatoPassaEPopolaIlContext(t *testing.T) {
	api := newTestAPI(t, New(&stubAuthorizer{allow: true}, cfg(true)))

	resp := api.Get("/probe", "X-Roles: OP, ADMIN ", "X-User: mrossi", "X-Context: NORD")
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, atteso 200: %s", resp.Code, resp.Body.String())
	}
	var got probe
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.User != "mrossi" || got.ContextID != "NORD" {
		t.Errorf("context = %+v", got)
	}
	if len(got.Roles) != 2 || got.Roles[0] != "OP" || got.Roles[1] != "ADMIN" {
		t.Errorf("ruoli = %v: la lista va ripulita dagli spazi", got.Roles)
	}
}

func TestMiddleware_ContestoNonAutorizzatoNega(t *testing.T) {
	// Nessun ruolo sopravvive al filtro per contesto.
	api := newTestAPI(t, New(&stubAuthorizer{allow: true, contextRoles: []string{}}, cfg(true)))

	resp := api.Get("/probe", "X-Roles: OP", "X-Context: SUD")
	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, atteso 403", resp.Code)
	}
	var body map[string]string
	_ = json.Unmarshal(resp.Body.Bytes(), &body)
	if body["code"] != coreauth.CodeForbiddenCtx {
		t.Errorf("codice = %q, atteso %q: il rifiuto sul contesto si distingue da quello sui ruoli",
			body["code"], coreauth.CodeForbiddenCtx)
	}
}

// Senza Authorizer il middleware nega. Un componente di sicurezza che, non sapendo decidere,
// lascia passare non si distingue da un'API che non protegge nulla.
func TestMiddleware_SenzaAuthorizerNega(t *testing.T) {
	api := newTestAPI(t, New(nil, cfg(true)))

	resp := api.Get("/probe", "X-Roles: ADMIN")
	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, atteso 403: senza authorizer non si autorizza", resp.Code)
	}
}

func TestMiddleware_GuestPathPassaSenzaRuoli(t *testing.T) {
	api := newTestAPI(t, New(&stubAuthorizer{allow: false}, cfg(true, "/probe")))

	if resp := api.Get("/probe"); resp.Code != http.StatusOK {
		t.Fatalf("status = %d, atteso 200 su una rotta guest", resp.Code)
	}
}

func TestMiddleware_PreflightPassa(t *testing.T) {
	api := newTestAPI(t, New(&stubAuthorizer{allow: false}, cfg(true)))

	resp := api.Do(http.MethodOptions, "/probe")
	if resp.Code == http.StatusForbidden {
		t.Error("la preflight CORS non porta credenziali: non va rifiutata dall'autorizzazione")
	}
}

func TestNew_DisabilitatoNonCostruisceNulla(t *testing.T) {
	if m := New(&stubAuthorizer{}, cfg(false)); m != nil {
		t.Error("con enabled=false il middleware non deve esistere")
	}
	if m := New(&stubAuthorizer{}, nil); m != nil {
		t.Error("senza sezione middleware il middleware non deve esistere")
	}
	// Register su un middleware nullo non deve esplodere: è il caso normale di un'app senza
	// autorizzazione, e il router lo chiama comunque.
	(*Middleware)(nil).Register(nil, nil)
}

func TestToken_RichiedeAppId(t *testing.T) {
	api := newTestAPI(t, New(&stubAuthorizer{allow: true}, cfg(true)))

	if resp := api.Get(TokenPath, "X-Roles: ADMIN"); resp.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, attesa 422: l'header AppId è obbligatorio", resp.Code)
	}
}

func TestToken_RestituisceTestoCifrato(t *testing.T) {
	api := newTestAPI(t, New(&stubAuthorizer{allow: true}, cfg(true)))

	resp := api.Get(TokenPath, "X-Roles: ADMIN", "X-User: mrossi", "AppId: APP_TEST")
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	if body == "" {
		t.Fatal("corpo vuoto")
	}
	if len(body) < 32 {
		t.Errorf("corpo = %q: atteso testo cifrato in esadecimale", body)
	}
}
