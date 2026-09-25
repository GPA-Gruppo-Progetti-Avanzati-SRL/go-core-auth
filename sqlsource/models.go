package sqlsource

import "github.com/uptrace/bun"

// Lo schema dell'ACL su SQL: cinque tabelle di entità e tre di associazione.
//
// I nomi li fissa la libreria, come go-core-sql/locker fa con scheduler_locks: lo schema in cui
// vivono lo sceglie la connessione, che è dove quella scelta è già espressa.
//
// `ord` e non `order`: è parola riservata in SQL e obbligherebbe a quotarla in ogni query.

type contextRow struct {
	bun.BaseModel `bun:"table:acl_context"`

	ID          string `bun:"id,pk"`
	Label       string `bun:"label"`
	Description string `bun:"description"`
	Icon        string `bun:"icon"`
	Ord         int    `bun:"ord"`
	// HomeApp designa l'app home del contesto. Vuoto = ricade sull'app con base_path "/".
	HomeApp string `bun:"home_app"`
}

type appRow struct {
	bun.BaseModel `bun:"table:acl_app"`

	ID          string `bun:"id,pk"`
	Description string `bun:"description"`
	BasePath    string `bun:"base_path"`
	Icon        string `bun:"icon"`
	Ord         int    `bun:"ord"`
}

type capabilityRow struct {
	bun.BaseModel `bun:"table:acl_capability"`

	ID          string `bun:"id,pk"`
	Category    string `bun:"category"`
	Description string `bun:"description"`

	// categoria "api"
	OperationID string `bun:"operation_id"`
	APIPath     string `bun:"api_path"`
	// APIMethods è una lista separata da virgola e non un array del dialetto: un array nativo
	// esiste su PostgreSQL e non altrove, e questa libreria non sceglie il dialetto.
	APIMethods string `bun:"api_methods"`

	// categoria "ui"
	Endpoint string `bun:"endpoint"`
	Icon     string `bun:"icon"`
	Ord      int    `bun:"ord"`
	Menu     bool   `bun:"menu"`

	// AppID vuoto = la capability vale per ogni app.
	AppID string `bun:"app_id"`
}

type capabilityGroupRow struct {
	bun.BaseModel `bun:"table:acl_capability_group"`

	ID          string `bun:"id,pk"`
	Description string `bun:"description"`
}

type capabilityGroupItemRow struct {
	bun.BaseModel `bun:"table:acl_capability_group_item"`

	GroupID      string `bun:"group_id,pk"`
	CapabilityID string `bun:"capability_id,pk"`
}

type roleRow struct {
	bun.BaseModel `bun:"table:acl_role"`

	ID string `bun:"id,pk"`
	// ContextID vuoto = ruolo context-agnostic.
	ContextID   string `bun:"context_id"`
	Description string `bun:"description"`
}

type roleGroupRow struct {
	bun.BaseModel `bun:"table:acl_role_group"`

	RoleID  string `bun:"role_id,pk"`
	GroupID string `bun:"group_id,pk"`
}

type roleCapRow struct {
	bun.BaseModel `bun:"table:acl_role_cap"`

	RoleID       string `bun:"role_id,pk"`
	CapabilityID string `bun:"capability_id,pk"`
}

// tables elenca i modelli nell'ordine in cui vanno create le tabelle (le associazioni per ultime).
func tables() []any {
	return []any{
		(*contextRow)(nil),
		(*appRow)(nil),
		(*capabilityRow)(nil),
		(*capabilityGroupRow)(nil),
		(*roleRow)(nil),
		(*capabilityGroupItemRow)(nil),
		(*roleGroupRow)(nil),
		(*roleCapRow)(nil),
	}
}
