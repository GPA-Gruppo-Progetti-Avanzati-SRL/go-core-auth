// Package apiauth espone l'autorizzazione come middleware HTTP e come endpoint del token di
// sessione. È il solo package di go-core-auth che dipende da huma: un'applicazione che non serve
// un'API non lo importa e non se lo porta nel grafo.
//
//	coreauth.Module(&svc.Auth,
//	    coreauth.WithSource(mongosource.Module),
//	    coreauth.WithMiddleware(apiauth.Module))
//
// Il montaggio sul router spetta a go-core-api, che è l'unico a possedere la huma.API.
package apiauth

import (
	"encoding/json"
	"net/http"
	"strings"

	coreauth "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-auth"
	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog/log"
)

// Middleware è ciò che go-core-api monta: il controllo di autorizzazione e l'endpoint del token.
type Middleware struct {
	authorizer coreauth.Authorizer
	cfg        *coreauth.MiddlewareConfig
	guestPaths map[string]struct{}
}

// New costruisce il middleware. Restituisce nil se l'autorizzazione è spenta in configurazione:
// un router che riceve nil non monta nulla.
func New(authorizer coreauth.Authorizer, cfg *coreauth.MiddlewareConfig) *Middleware {
	if cfg == nil || !cfg.Enabled {
		return nil
	}
	cfg.WithDefaults()

	guest := make(map[string]struct{}, len(cfg.GuestPaths))
	for _, p := range cfg.GuestPaths {
		guest[p] = struct{}{}
	}
	return &Middleware{authorizer: authorizer, cfg: cfg, guestPaths: guest}
}

// Register monta il middleware sulla huma.API e registra l'endpoint del token.
//
// withDefaults, se passata, arricchisce l'operazione con le response d'errore standard di chi
// ospita l'API: è la sola cosa che apiauth non può sapere, perché appartiene al vocabolario del
// framework HTTP e non a quello dell'autorizzazione.
func (m *Middleware) Register(api huma.API, withDefaults func(huma.Operation) huma.Operation) {
	if m == nil {
		return
	}
	api.UseMiddleware(m.handle)

	op := tokenOperation()
	if withDefaults != nil {
		op = withDefaults(op)
	}
	huma.Register(api, op, m.token)
}

// handle è il middleware: riconosce l'identità dalla richiesta, riduce i ruoli al contesto
// presentato e autorizza la rotta.
func (m *Middleware) handle(ctx huma.Context, next func(huma.Context)) {
	// La preflight CORS non porta credenziali e non raggiunge un handler applicativo.
	if strings.EqualFold(ctx.Method(), http.MethodOptions) {
		next(ctx)
		return
	}

	path := ctx.URL().Path
	if _, ok := m.guestPaths[path]; ok {
		next(ctx)
		return
	}

	rolesHeader := ctx.Header(m.cfg.RolesHeader)
	if strings.TrimSpace(rolesHeader) == "" {
		deny(ctx, coreauth.CodeForbiddenRole, "nessun ruolo presentato")
		return
	}

	allRoles := parseRoles(rolesHeader, m.cfg.Delimiter)
	user := strings.TrimSpace(ctx.Header(m.cfg.UserHeader))
	contextID := strings.TrimSpace(ctx.Header(m.cfg.ContextHeader))

	// Un middleware di autorizzazione senza authorizer non ha modo di autorizzare: nega. Lasciar
	// passare sarebbe l'unico caso in cui un guasto della configurazione allarga i permessi invece
	// di restringerli, e non si distinguerebbe da un'API che non protegge nulla.
	if m.authorizer == nil {
		log.Error().Str("path", path).
			Msg("autorizzazione: nessun Authorizer disponibile, la richiesta è rifiutata")
		deny(ctx, coreauth.CodeForbiddenRole, "autorizzazione non disponibile")
		return
	}

	contextRoles := allRoles
	if contextID != "" {
		contextRoles = m.authorizer.FilterRolesByContext(allRoles, contextID)
		if len(contextRoles) == 0 {
			deny(ctx, coreauth.CodeForbiddenCtx, "contesto non autorizzato")
			return
		}
	}

	if !m.authorizer.MatchRequest(contextRoles, path, ctx.Method()) {
		deny(ctx, coreauth.CodeForbiddenRole, "nessun ruolo abilita questa rotta")
		return
	}

	ctx = huma.WithValue(ctx, keyUser, user)
	ctx = huma.WithValue(ctx, keyContextID, contextID)
	ctx = huma.WithValue(ctx, keyRoles, contextRoles)
	ctx = huma.WithValue(ctx, keyAllRoles, allRoles)
	ctx = huma.WithValue(ctx, keyAuthorizer, m.authorizer)
	next(ctx)
}

func parseRoles(v, delimiter string) []string {
	parts := strings.Split(v, delimiter)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// deny scrive il 403 nella stessa forma delle altre risposte d'errore (ambit/code/message): un
// rifiuto di autorizzazione è un errore applicativo come gli altri, e un client non deve avere un
// ramo apposta per trattarlo.
func deny(ctx huma.Context, code, message string) {
	ctx.SetStatus(http.StatusForbidden)
	ctx.SetHeader("Content-Type", "application/json")
	body, _ := json.Marshal(struct {
		Ambit   string `json:"ambit"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}{coreauth.Ambit, code, message})
	_, _ = ctx.BodyWriter().Write(body)
}
