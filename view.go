package coreauth

import (
	"sort"
	"strings"
)

// view è lo stato risolto dell'ACL: immutabile per costruzione e sostituito in blocco a ogni
// refresh. È ciò che rende una lettura concorrente coerente — o vede tutta la vecchia o tutta la
// nuova, mai un misto — e ciò che fa sparire davvero un ruolo revocato, che in uno stato
// aggiornato per chiavi resterebbe attivo finché il processo non riparte.
type view struct {
	roleCaps      map[string][]*Capability // roleID → capability già espanse da gruppi + dirette
	roleContext   map[string]string        // roleID → contesto normalizzato ("" = agnostic)
	agnosticRoles map[string]struct{}
	apps          []*AppDef              // ordinate per Order, poi ID
	contexts      map[string]*ContextDef // id normalizzato → definizione
	contextList   []*ContextDef          // ordinate per Order, poi ID
	homeApps      map[string]string      // contesto normalizzato → appID designata
	defaultHome   string                 // home da usare quando nessun contesto la designa
}

// normCtx normalizza un id di contesto per l'uso come chiave.
//
// Gli id di contesto arrivano da tre strade — l'ACL, l'header X-Context e le rotte /{cid}/ — e le
// tre non concordano sul case. Normalizzare all'ingresso è ciò che evita di doverlo ricordare a
// ogni lookup: dimenticarlo in un punto solo significa una home che non si trova mai.
func normCtx(id string) string { return strings.ToUpper(strings.TrimSpace(id)) }

// emptyView è lo stato di chi non ha ancora caricato nulla: nega tutto.
func emptyView() *view {
	return &view{
		roleCaps:      map[string][]*Capability{},
		roleContext:   map[string]string{},
		agnosticRoles: map[string]struct{}{},
		contexts:      map[string]*ContextDef{},
		homeApps:      map[string]string{},
	}
}

// newView risolve uno Snapshot nella forma che l'engine interroga: espande i capability-group dei
// ruoli, normalizza i contesti, ordina cataloghi e capability.
//
// Il secondo valore elenca i riferimenti pendenti trovati (un ruolo che nomina un gruppo o una
// capability inesistente): non sono un errore — l'ACL resta utilizzabile — ma sono un difetto di
// configurazione che senza una segnalazione si manifesta solo come un permesso che non arriva.
func newView(s *Snapshot) (*view, []string) {
	v := emptyView()
	var dangling []string

	caps := make(map[string]*Capability, len(s.Capabilities))
	for i := range s.Capabilities {
		c := &s.Capabilities[i]
		if c.ID != "" {
			caps[c.ID] = c
		}
	}

	groups := make(map[string][]string, len(s.CapabilityGroups))
	for _, g := range s.CapabilityGroups {
		if g.ID != "" {
			groups[g.ID] = g.Capabilities
		}
	}

	// --- contesti ---
	for i := range s.Contexts {
		c := &s.Contexts[i]
		if c.ID == "" {
			continue
		}
		key := normCtx(c.ID)
		v.contexts[key] = c
		v.contextList = append(v.contextList, c)
		if c.HomeApp != "" {
			v.homeApps[key] = c.HomeApp
		}
	}
	sortDefs(v.contextList, func(c *ContextDef) (int, string) { return c.Order, c.ID })

	// --- app ---
	designated := make(map[string]struct{}, len(v.homeApps))
	for _, appID := range v.homeApps {
		designated[appID] = struct{}{}
	}
	for i := range s.Apps {
		a := &s.Apps[i]
		if a.ID != "" {
			v.apps = append(v.apps, a)
		}
	}
	sortDefs(v.apps, func(a *AppDef) (int, string) { return a.Order, a.ID })

	// defaultHome: la home da mostrare quando il contesto non ne designa una. Si preferisce una
	// home non assegnata ad alcun contesto — è quella "generica" — e solo in sua assenza la prima
	// con BasePath "/", perché un ACL in cui ogni home è designata resti comunque navigabile.
	for _, a := range v.apps {
		if a.BasePath != "/" {
			continue
		}
		if _, isDesignated := designated[a.ID]; !isDesignated {
			v.defaultHome = a.ID
			break
		}
		if v.defaultHome == "" {
			v.defaultHome = a.ID
		}
	}

	// --- ruoli: espansione dei gruppi + capability dirette ---
	for _, r := range s.Roles {
		if r.ID == "" {
			continue
		}
		key := normCtx(r.ContextID)
		v.roleContext[r.ID] = key
		if key == "" {
			v.agnosticRoles[r.ID] = struct{}{}
		}

		seen := make(map[string]struct{}, len(r.Capabilities))
		var resolved []*Capability
		add := func(capID string) {
			if _, done := seen[capID]; done {
				return
			}
			seen[capID] = struct{}{}
			c, ok := caps[capID]
			if !ok {
				dangling = append(dangling, "role "+r.ID+" → capability "+capID)
				return
			}
			resolved = append(resolved, c)
		}
		for _, gid := range r.CapabilityGroups {
			g, ok := groups[gid]
			if !ok {
				dangling = append(dangling, "role "+r.ID+" → capability-group "+gid)
				continue
			}
			for _, capID := range g {
				add(capID)
			}
		}
		for _, capID := range r.Capabilities {
			add(capID)
		}
		sort.Slice(resolved, func(i, j int) bool { return resolved[i].ID < resolved[j].ID })
		v.roleCaps[r.ID] = resolved
	}

	return v, dangling
}

// sortDefs ordina un catalogo per (Order, ID): l'ordine dichiarato nell'ACL, con l'id a rompere i
// pari. Serve a rendere deterministico ciò che l'Authorizer restituisce.
func sortDefs[T any](s []*T, key func(*T) (int, string)) {
	sort.Slice(s, func(i, j int) bool {
		oi, ni := key(s[i])
		oj, nj := key(s[j])
		if oi != oj {
			return oi < oj
		}
		return ni < nj
	})
}

// filterRoles è il corpo di FilterRolesByContext, su una view già in mano al chiamante.
func (v *view) filterRoles(roles []string, key string) []string {
	if key == "" {
		return roles
	}
	out := make([]string, 0, len(roles))
	for _, r := range roles {
		if _, agnostic := v.agnosticRoles[r]; agnostic {
			out = append(out, r)
			continue
		}
		if rc, ok := v.roleContext[r]; ok && rc == key {
			out = append(out, r)
		}
	}
	return out
}

// homeAppFor risolve la home app del contesto indicato.
//
// Un contesto che non esiste nell'ACL non ha home: non ricade sulla generica. La differenza conta
// perché un contesto rimosso continuerebbe altrimenti a rispondere con un'app plausibile, cioè a
// non sembrare rimosso.
func (v *view) homeAppFor(key string) string {
	if key == "" {
		return v.defaultHome
	}
	c, ok := v.contexts[key]
	if !ok {
		return ""
	}
	if c.HomeApp != "" {
		return c.HomeApp
	}
	return v.defaultHome
}

// eachCap applica fn a ogni capability dei ruoli indicati, una sola volta per capability.
func (v *view) eachCap(roles []string, category string, fn func(*Capability) bool) {
	seen := make(map[string]struct{})
	for _, r := range roles {
		for _, c := range v.roleCaps[r] {
			if c.Category != category {
				continue
			}
			if _, done := seen[c.ID]; done {
				continue
			}
			seen[c.ID] = struct{}{}
			if !fn(c) {
				return
			}
		}
	}
}

// scopedToApp dice se una capability vale per l'app indicata. Una capability senza app-id è
// globale e vale per ogni app: la regola è la stessa per i menu e per le azioni, che è ciò che
// permette di dichiarare una voce comune una volta sola.
func scopedToApp(c *Capability, appID string) bool {
	return c.AppID == "" || c.AppID == appID
}
