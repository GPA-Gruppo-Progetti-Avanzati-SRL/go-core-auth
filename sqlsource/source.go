// Package sqlsource legge l'ACL da un database relazionale: è il backend sql di go-core-auth.
//
//	coreauth.Module(&svc.Auth, coreauth.WithSource(sqlsource.Module))
//
// Lo schema è in models.go — cinque tabelle di entità e tre di associazione — ed è lo specchio
// esatto del modello che mongosource legge da documenti: le due sorgenti producono lo stesso
// Snapshot, quindi la scelta del backend non cambia le decisioni di autorizzazione.
//
// Le tabelle le crea EnsureTables, oppure la migrazione dell'applicazione.
package sqlsource

import (
	"context"
	"strings"

	core "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	coreauth "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-auth"
	"github.com/uptrace/bun"
)

// Ambit e codici degli errori di questo backend.
const (
	Ambit         = "go-core-auth/sqlsource"
	CodeSelect    = "AUTH-SQL-SELECT"
	CodeEnsureDDL = "AUTH-SQL-DDL"
)

type source struct {
	db bun.IDB
}

// Load legge le otto tabelle dentro un'unica transazione di sola lettura.
//
// La transazione non serve a scrivere: serve a leggere uno stato solo. Senza, le associazioni
// potrebbero essere lette dopo una modifica che le entità non hanno visto, e lo Snapshot
// descriverebbe un ACL che non è mai esistito — un ruolo che referenzia un gruppo appena
// cancellato, o una capability che nessuno possiede più.
func (s *source) Load(ctx context.Context) (*coreauth.Snapshot, *core.ApplicationError) {
	var (
		contexts   []contextRow
		apps       []appRow
		caps       []capabilityRow
		groups     []capabilityGroupRow
		groupItems []capabilityGroupItemRow
		roles      []roleRow
		roleGroups []roleGroupRow
		roleCaps   []roleCapRow
	)

	err := s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		for _, dst := range []any{
			&contexts, &apps, &caps, &groups, &groupItems, &roles, &roleGroups, &roleCaps,
		} {
			if err := tx.NewSelect().Model(dst).Scan(ctx); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, core.TechnicalError().WithAmbit(Ambit).WithCode(CodeSelect).WithCause(err)
	}

	return toSnapshot(contexts, apps, caps, groups, groupItems, roles, roleGroups, roleCaps), nil
}

// toSnapshot ricompone le righe nello Snapshot: le associazioni tornano a essere liste di id sulle
// entità che le possiedono, che è la forma in cui l'engine le espande.
func toSnapshot(
	contexts []contextRow, apps []appRow, caps []capabilityRow,
	groups []capabilityGroupRow, groupItems []capabilityGroupItemRow,
	roles []roleRow, roleGroups []roleGroupRow, roleCaps []roleCapRow,
) *coreauth.Snapshot {

	snap := &coreauth.Snapshot{}

	for _, c := range contexts {
		snap.Contexts = append(snap.Contexts, coreauth.ContextDef{
			ID: c.ID, Label: c.Label, Description: c.Description,
			Icon: c.Icon, Order: c.Ord, HomeApp: c.HomeApp,
		})
	}
	for _, a := range apps {
		snap.Apps = append(snap.Apps, coreauth.AppDef{
			ID: a.ID, Description: a.Description, BasePath: a.BasePath,
			Icon: a.Icon, Order: a.Ord,
		})
	}
	for _, c := range caps {
		snap.Capabilities = append(snap.Capabilities, coreauth.Capability{
			ID: c.ID, Category: c.Category, Description: c.Description,
			OperationID: c.OperationID, APIPath: c.APIPath, APIMethods: splitMethods(c.APIMethods),
			Endpoint: c.Endpoint, Icon: c.Icon, Order: c.Ord, Menu: c.Menu, AppID: c.AppID,
		})
	}

	byGroup := index(groupItems, func(r capabilityGroupItemRow) (string, string) {
		return r.GroupID, r.CapabilityID
	})
	for _, g := range groups {
		snap.CapabilityGroups = append(snap.CapabilityGroups, coreauth.CapabilityGroup{
			ID: g.ID, Description: g.Description, Capabilities: byGroup[g.ID],
		})
	}

	groupsByRole := index(roleGroups, func(r roleGroupRow) (string, string) { return r.RoleID, r.GroupID })
	capsByRole := index(roleCaps, func(r roleCapRow) (string, string) { return r.RoleID, r.CapabilityID })
	for _, r := range roles {
		snap.Roles = append(snap.Roles, coreauth.Role{
			ID: r.ID, ContextID: r.ContextID, Description: r.Description,
			CapabilityGroups: groupsByRole[r.ID], Capabilities: capsByRole[r.ID],
		})
	}

	return snap
}

// index raggruppa le righe di una tabella di associazione per il loro lato sinistro.
func index[T any](rows []T, key func(T) (string, string)) map[string][]string {
	out := make(map[string][]string)
	for _, r := range rows {
		k, v := key(r)
		out[k] = append(out[k], v)
	}
	return out
}

// splitMethods legge la lista di metodi HTTP. Vuota significa "tutti i metodi", quindi una stringa
// vuota deve restare una slice vuota e non una slice con un elemento vuoto, che non matcherebbe
// nulla.
func splitMethods(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// EnsureTables crea le tabelle dell'ACL se non esistono.
//
// È esplicita e non automatica al boot: creare tabelle è una modifica allo schema, e in un
// ambiente con migrazioni gestite deve restare una scelta di chi le governa. Le applicazioni che
// non ne hanno la chiamano una volta all'avvio.
func EnsureTables(ctx context.Context, db *bun.DB) *core.ApplicationError {
	for _, model := range tables() {
		if _, err := db.NewCreateTable().Model(model).IfNotExists().Exec(ctx); err != nil {
			return core.TechnicalError().WithAmbit(Ambit).WithCode(CodeEnsureDDL).WithCause(err)
		}
	}
	return nil
}
