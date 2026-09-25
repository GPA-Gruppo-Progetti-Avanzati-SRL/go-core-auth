// Package yamlsource legge l'ACL da un file YAML: è il backend senza database di go-core-auth.
//
//	coreauth.Module(&svc.Auth, coreauth.WithSource(yamlsource.Module))
//
// Il file contiene l'ACL intero (contesti, app, capability, gruppi, ruoli) ed è riletto a ogni
// ricaricamento: modificarlo e attendere l'intervallo di refresh basta a cambiare i permessi, senza
// riavviare il processo.
//
// Un Catalog supplito dall'applicazione aggiunge le entità che il file non può conoscere — le
// capability scoperte al boot dai manifest dei frontend e dai file generati dai backend. In
// conflitto sullo stesso id vince il file: è la dichiarazione esplicita.
package yamlsource

import (
	"context"
	"os"

	core "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	coreauth "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-auth"
	"go.yaml.in/yaml/v3"
)

// Ambit e codici degli errori di questo backend.
const (
	Ambit          = "go-core-auth/yamlsource"
	CodeNoRules    = "AUTH-YAML-NOFILE"
	CodeReadRules  = "AUTH-YAML-READ"
	CodeParseRules = "AUTH-YAML-PARSE"
)

// Catalog è ciò che l'applicazione aggiunge al file: le entità che scopre al boot e che nel file
// non possono stare, tipicamente le capability dichiarate dai frontend e dai backend registrati.
//
// Si fornisce a fx prima di coreauth.Module:
//
//	core.Supply(yamlsource.Catalog{Apps: apps, Contexts: ctxs, Capabilities: discovered})
type Catalog struct {
	Contexts     []coreauth.ContextDef
	Apps         []coreauth.AppDef
	Capabilities []coreauth.Capability
}

type source struct {
	path    string
	catalog Catalog
}

// Load rilegge il file e lo fonde col catalogo.
func (s *source) Load(context.Context) (*coreauth.Snapshot, *core.ApplicationError) {
	if s.path == "" {
		return nil, core.TechnicalError().WithAmbit(Ambit).WithCode(CodeNoRules).
			WithMessage("nessun file delle regole configurato: valorizzare services.auth.yaml.rules-file")
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, core.TechnicalError().WithAmbit(Ambit).WithCode(CodeReadRules).WithCause(err)
	}

	var rules rulesFile
	if err := yaml.Unmarshal(data, &rules); err != nil {
		return nil, core.TechnicalError().WithAmbit(Ambit).WithCode(CodeParseRules).WithCause(err)
	}

	return merge(rules, s.catalog), nil
}

// merge compone lo Snapshot: prima il catalogo, poi il file, che sovrascrive per id.
func merge(rules rulesFile, cat Catalog) *coreauth.Snapshot {
	snap := &coreauth.Snapshot{}

	snap.Contexts = mergeByID(cat.Contexts, toContexts(rules.Contexts),
		func(c coreauth.ContextDef) string { return c.ID })
	snap.Apps = mergeByID(cat.Apps, toApps(rules.Apps),
		func(a coreauth.AppDef) string { return a.ID })
	snap.Capabilities = mergeByID(cat.Capabilities, toCapabilities(rules.Capabilities),
		func(c coreauth.Capability) string { return c.ID })

	for _, g := range rules.CapabilityGroups {
		if g.ID == "" {
			continue
		}
		snap.CapabilityGroups = append(snap.CapabilityGroups, coreauth.CapabilityGroup{
			ID: g.ID, Description: g.Description, Capabilities: g.Capabilities,
		})
	}
	for _, r := range rules.Roles {
		if r.ID == "" {
			continue
		}
		snap.Roles = append(snap.Roles, coreauth.Role{
			ID: r.ID, ContextID: r.Context, Description: r.Description,
			CapabilityGroups: r.CapabilityGroups, Capabilities: r.Capabilities,
		})
	}

	return snap
}

// mergeByID sovrappone override a base conservando l'ordine: gli elementi di base restano al loro
// posto (sostituiti se ridichiarati), i nuovi si accodano. L'ordine stabile conta perché i
// cataloghi si mostrano all'utente.
func mergeByID[T any](base, override []T, id func(T) string) []T {
	pos := make(map[string]int, len(base))
	out := make([]T, 0, len(base)+len(override))
	for _, b := range base {
		if id(b) == "" {
			continue
		}
		pos[id(b)] = len(out)
		out = append(out, b)
	}
	for _, o := range override {
		key := id(o)
		if key == "" {
			continue
		}
		if i, ok := pos[key]; ok {
			out[i] = o
			continue
		}
		pos[key] = len(out)
		out = append(out, o)
	}
	return out
}

func toContexts(in []contextRule) []coreauth.ContextDef {
	out := make([]coreauth.ContextDef, 0, len(in))
	for _, c := range in {
		out = append(out, coreauth.ContextDef{
			ID: c.ID, Label: c.Label, Description: c.Description,
			Icon: c.Icon, Order: c.Order, HomeApp: c.HomeApp,
		})
	}
	return out
}

func toApps(in []appRule) []coreauth.AppDef {
	out := make([]coreauth.AppDef, 0, len(in))
	for _, a := range in {
		out = append(out, coreauth.AppDef{
			ID: a.ID, Description: a.Description, BasePath: a.BasePath,
			Icon: a.Icon, Order: a.Order,
		})
	}
	return out
}

func toCapabilities(in []capabilityRule) []coreauth.Capability {
	out := make([]coreauth.Capability, 0, len(in))
	for _, c := range in {
		item := coreauth.Capability{
			ID: c.ID, Category: c.Category, Description: c.Description, AppID: c.AppID,
		}
		if c.API != nil {
			item.OperationID = c.API.OperationID
			item.APIPath = c.API.Path
			item.APIMethods = c.API.Methods
		}
		if c.UI != nil {
			item.Endpoint = c.UI.Endpoint
			item.Icon = c.UI.Icon
			item.Order = c.UI.Order
			item.Menu = c.UI.Menu
		}
		out = append(out, item)
	}
	return out
}
