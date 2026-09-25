package coreauth

import (
	"context"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	core "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	"github.com/rs/zerolog/log"
)

// lut è l'implementazione di Authorizer: una view risolta, sostituita in blocco quando scade.
//
// Il ricaricamento è lazy-on-read — lo innesca il primo lettore che trova la view scaduta, in
// background, e quel lettore risponde con la view che ha — e non un ticker: un processo senza
// traffico non deve interrogare il backend dell'ACL per mantenere aggiornato qualcosa che nessuno
// sta leggendo.
type lut struct {
	src     Source
	refresh time.Duration
	timeout time.Duration

	mu       sync.Mutex
	updating atomic.Bool
	cur      atomic.Pointer[view]
	loadedAt atomic.Int64 // unix nano dell'ultimo caricamento riuscito
}

func newLUT(src Source, refresh, timeout time.Duration) *lut {
	l := &lut{src: src, refresh: refresh, timeout: timeout}
	l.cur.Store(emptyView())
	return l
}

// load esegue un caricamento e, se riesce, sostituisce la view.
//
// Non c'è aggiornamento parziale: o lo Snapshot arriva intero e la view nuova prende il posto
// della vecchia, o la vecchia resta esattamente com'era. È l'invariante che rende osservabile una
// revoca (ciò che non è più nell'ACL sparisce) e innocuo un backend momentaneamente illeggibile
// (si continua a decidere sullo snapshot precedente, e il log lo dice).
func (l *lut) load(ctx context.Context) *core.ApplicationError {
	l.mu.Lock()
	if l.updating.Load() {
		l.mu.Unlock()
		return nil
	}
	l.updating.Store(true)
	l.mu.Unlock()
	defer l.updating.Store(false)

	ctx, cancel := context.WithTimeout(ctx, l.timeout)
	defer cancel()

	snap, err := l.src.Load(ctx)
	if err != nil {
		return err
	}
	if snap == nil {
		return core.TechnicalError().WithAmbit(Ambit).WithCode(CodeSourceLoad).
			WithMessage("la sorgente ACL ha restituito uno snapshot nullo senza errore")
	}

	v, dangling := newView(snap)
	l.cur.Store(v)
	l.loadedAt.Store(time.Now().UnixNano())

	ev := log.Info().Int("roles", len(v.roleCaps)).Int("apps", len(v.apps)).
		Int("contexts", len(v.contextList)).Int("capabilities", len(snap.Capabilities))
	ev.Msg("ACL caricato")
	if len(dangling) > 0 {
		// Un riferimento pendente non invalida l'ACL, ma è un permesso che non arriverà mai a
		// destinazione: senza questa riga il sintomo è un 403 che nessuna configurazione spiega.
		log.Warn().Int("count", len(dangling)).Strs("sample", sample(dangling, 10)).
			Msg("ACL: riferimenti pendenti, le capability corrispondenti non sono state assegnate")
	}
	return nil
}

func sample(s []string, n int) []string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// expired dice se la view corrente ha superato l'intervallo di refresh.
func (l *lut) expired() bool {
	if l.updating.Load() {
		return false
	}
	return time.Since(time.Unix(0, l.loadedAt.Load())) > l.refresh
}

// current restituisce la view su cui rispondere e, se è scaduta, ne fa ricaricare una.
//
// trigger nomina il lettore che ha innescato il ricaricamento: senza, dal log non si ricostruisce
// quale strada abbia fatto scattare una lettura del backend.
func (l *lut) current(trigger string) *view {
	if l.expired() {
		go func() {
			if err := l.load(context.Background()); err != nil {
				// Il lettore ha già risposto con lo snapshot che aveva, quindi l'errore non ha un
				// destinatario — ma dice che le decisioni di autorizzazione proseguono su dati
				// vecchi, che è esattamente ciò che non si vuole scoprire dopo.
				log.Warn().Err(err).Str("trigger", trigger).
					Msg("ACL: ricaricamento in background fallito, resta lo snapshot precedente")
			}
		}()
	}
	return l.cur.Load()
}

// --- Authorizer ---

func (l *lut) FilterRolesByContext(roles []string, contextID string) []string {
	return l.current("FilterRolesByContext").filterRoles(roles, normCtx(contextID))
}

// GetContexts deriva i contesti accessibili dai ruoli dell'utente. Un ruolo context-agnostic vale
// su ogni contesto e quindi li espone tutti: la lettura opposta lascerebbe senza alcun contesto
// selezionabile proprio l'utente autorizzato ovunque.
func (l *lut) GetContexts(roles []string) []*Context {
	v := l.current("GetContexts")

	for _, r := range roles {
		if _, ok := v.agnosticRoles[r]; ok {
			out := make([]*Context, 0, len(v.contextList))
			for _, c := range v.contextList {
				out = append(out, toContext(c))
			}
			return out
		}
	}

	seen := make(map[string]struct{})
	out := make([]*Context, 0, len(roles))
	for _, c := range v.contextList {
		key := normCtx(c.ID)
		if _, done := seen[key]; done {
			continue
		}
		for _, r := range roles {
			if rc, ok := v.roleContext[r]; ok && rc != "" && rc == key {
				seen[key] = struct{}{}
				out = append(out, toContext(c))
				break
			}
		}
	}
	return out
}

func toContext(c *ContextDef) *Context {
	return &Context{ID: c.ID, Description: c.Description, Label: c.Label, Icon: c.Icon, Order: c.Order}
}

// GetApps compone il catalogo navigabile.
//
// La home del contesto è quella che il contesto designa (ContextDef.HomeApp), non quella che si
// indovina dal path: il path resta il fallback per un ACL che non designa nulla.
//
// Con contesti dichiarati e nessuno selezionato si espone la sola home — è la pagina da cui si
// sceglie il contesto. Un ACL senza contesti non ha quella fase, e le app si espongono subito coi
// loro path: la distinzione la fa la forma dell'ACL, non i ruoli di chi chiede, così lo stesso
// deployment si comporta allo stesso modo per tutti.
func (l *lut) GetApps(roles []string, contextID string) []*App {
	v := l.current("GetApps")
	key := normCtx(contextID)

	accessible := make(map[string]struct{})
	v.eachCap(v.filterRoles(roles, key), CategoryUI, func(c *Capability) bool {
		if c.AppID != "" {
			accessible[c.AppID] = struct{}{}
		}
		return true
	})

	home := v.homeAppFor(key)
	selecting := key == "" && len(v.contextList) > 0
	cid := strings.ToLower(contextID)

	out := make([]*App, 0, len(v.apps))
	for _, a := range v.apps {
		isHome := a.ID == home
		if !isHome {
			if selecting {
				continue
			}
			if _, ok := accessible[a.ID]; !ok {
				continue
			}
		}

		path, appCtx := a.BasePath, ""
		if key != "" {
			if isHome {
				path = "/" + cid + "/"
			} else {
				path = "/" + cid + a.BasePath
			}
			appCtx = contextID
		}
		out = append(out, &App{
			ID: a.ID, Description: a.Description, Path: path,
			Icon: a.Icon, Order: a.Order, ContextID: appCtx,
		})
	}
	return out
}

func (l *lut) GetPaths(roles []string, appID string) []*Path {
	v := l.current("GetPaths")
	out := make([]*Path, 0, 8)
	v.eachCap(roles, CategoryUI, func(c *Capability) bool {
		if scopedToApp(c, appID) {
			out = append(out, &Path{
				ID: c.ID, Description: c.Description, Icon: c.Icon,
				Order: c.Order, Menu: c.Menu, Endpoint: c.Endpoint,
			})
		}
		return true
	})
	sort.Slice(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func (l *lut) GetCapabilities(roles []string, appID string) []string {
	v := l.current("GetCapabilities")
	out := make([]string, 0, 8)
	v.eachCap(roles, CategoryActionUI, func(c *Capability) bool {
		if scopedToApp(c, appID) {
			out = append(out, c.ID)
		}
		return true
	})
	sort.Strings(out)
	return out
}

func (l *lut) GetServerCapabilities(roles []string) []string {
	v := l.current("GetServerCapabilities")
	out := make([]string, 0, 8)
	v.eachCap(roles, CategoryAPI, func(c *Capability) bool {
		out = append(out, c.ID)
		return true
	})
	sort.Strings(out)
	return out
}

func (l *lut) HasCapability(roles []string, capabilityID string) bool {
	v := l.current("HasCapability")
	found := false
	v.eachCap(roles, CategoryActionAPI, func(c *Capability) bool {
		if c.ID == capabilityID {
			found = true
			return false
		}
		return true
	})
	return found
}

func (l *lut) MatchRequest(roles []string, path, method string) bool {
	v := l.current("MatchRequest")
	ok := false
	v.eachCap(roles, CategoryAPI, func(c *Capability) bool {
		if c.APIPath == "" {
			return true
		}
		if matchGlob(c.APIPath, path) && matchMethods(c.APIMethods, method) {
			ok = true
			return false
		}
		return true
	})
	return ok
}

func (l *lut) AllContextIDs() []string {
	v := l.current("AllContextIDs")
	out := make([]string, 0, len(v.contextList))
	for _, c := range v.contextList {
		out = append(out, c.ID)
	}
	return out
}

func (l *lut) HomeAppForContext(contextID string) string {
	return l.current("HomeAppForContext").homeAppFor(normCtx(contextID))
}
