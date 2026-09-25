package coreauth

import (
	"context"

	core "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
)

// Source è il seam dei backend ACL: l'unica cosa che un backend implementa.
//
// Load legge l'ACL e ne restituisce lo stato completo. È atomico per contratto: o produce uno
// Snapshot coerente, o ritorna errore. Una lettura parziale non va restituita come successo —
// l'engine sostituisce la propria view in blocco, quindi uno Snapshot monco diventa una revoca di
// massa silenziosa.
type Source interface {
	Load(ctx context.Context) (*Snapshot, *core.ApplicationError)
}

// SourceFunc adatta una funzione a Source, per i backend che non hanno stato.
type SourceFunc func(ctx context.Context) (*Snapshot, *core.ApplicationError)

func (f SourceFunc) Load(ctx context.Context) (*Snapshot, *core.ApplicationError) { return f(ctx) }
