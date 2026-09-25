package coreauth_test

import (
	"context"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-auth/apiauth"
	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-auth/sqlsource"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
)

// Il seed delle capability lo genera go-core-api dal registry huma; le tabelle e la collection le
// leggono i backend di go-core-auth. Sono due moduli rilasciati separatamente e nulla li allinea a
// compile-time — una colonna rinominata da un lato è una query che in produzione non trova la
// colonna.
//
// Il presidio sta nel package radice perché serializzatore (apiauth) e lettore (sqlsource) sono due
// sottopackage che non si conoscono: nessuno dei due può confrontarsi con l'altro dall'interno
// senza prendersene la dipendenza.

var insertRe = regexp.MustCompile(`(?s)INSERT INTO (\w+) \(([^)]*)\)`)

// apiConRotte costruisce una huma.API con qualche operazione registrata, che è ciò da cui il
// generatore ricava le capability.
func apiConRotte(t *testing.T) huma.API {
	t.Helper()
	cfg := huma.DefaultConfig("seed", "1.0.0")
	cfg.CreateHooks = nil // come il Router di coreapi: vedi apiauth/middleware_test.go
	_, api := humatest.New(t, cfg)

	huma.Get(api, "/api/persons", func(context.Context, *struct{}) (*struct{ Body []string }, error) {
		return nil, nil
	})
	huma.Post(api, "/api/persons", func(context.Context, *struct{}) (*struct{ Body string }, error) {
		return nil, nil
	})
	return api
}

func TestSeedSQL_ScriveSulleColonneCheSqlsourceLegge(t *testing.T) {
	script := apiauth.SeedSQL(apiConRotte(t))

	schema := sqlsource.Schema()
	matches := insertRe.FindAllStringSubmatch(script, -1)
	if len(matches) == 0 {
		t.Fatalf("il generatore non ha prodotto alcuna INSERT:\n%s", script)
	}

	visitate := map[string]bool{}
	for _, m := range matches {
		tabella, colonne := m[1], splitColumns(m[2])
		visitate[tabella] = true

		note, ok := schema[tabella]
		if !ok {
			t.Errorf("INSERT su %q, che non è una tabella di sqlsource: %v", tabella, sqlsource.TableNames())
			continue
		}
		for _, c := range colonne {
			if !slices.Contains(note, c) {
				t.Errorf("tabella %s: la colonna %q non esiste nel modello di sqlsource (%v)", tabella, c, note)
			}
		}
	}

	for _, attesa := range []string{"acl_capability", "acl_capability_group", "acl_capability_group_item"} {
		if !visitate[attesa] {
			t.Errorf("il seed non scrive su %s", attesa)
		}
	}
}

// Il seed mongo deve parlare la lingua che mongosource smista: _et maiuscolo e sottodocumento api
// sulle sole capability di categoria "api".
func TestSeedMongo_ParlaLaLinguaDiMongosource(t *testing.T) {
	script := apiauth.SeedMongo(apiConRotte(t))

	for _, attesa := range []string{`"_et": "CAPABILITY"`, `"_et": "CAPABILITYGROUP"`, `"operationid"`} {
		if !strings.Contains(script, attesa) {
			t.Errorf("il seed non contiene %s:\n%s", attesa, script)
		}
	}
	if strings.Contains(script, "cap-def") {
		t.Error("il seed per go-core-auth non deve usare il vocabolario del frontdoor OPEM")
	}
}

func splitColumns(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(strings.ReplaceAll(p, "\n", "")); t != "" {
			out = append(out, t)
		}
	}
	return out
}
