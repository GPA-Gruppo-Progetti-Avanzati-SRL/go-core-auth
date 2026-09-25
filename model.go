package coreauth

// Le quattro categorie di capability. La categoria decide chi legge la capability e con quale
// domanda: "api" autorizza una richiesta HTTP, "ui" è una voce di menu, "action_ui" abilita un
// comando nell'interfaccia, "action_api" un'azione nella business logic del backend.
const (
	CategoryAPI       = "api"
	CategoryUI        = "ui"
	CategoryActionUI  = "action_ui"
	CategoryActionAPI = "action_api"
)

// Snapshot è l'ACL a un istante, come una Source lo consegna: entità grezze, riferimenti NON
// risolti. L'espansione dei capability-group in capability effettive del ruolo è compito
// dell'engine, non della sorgente — è la regola che rende una sorgente nuova un fatto di query.
type Snapshot struct {
	Contexts         []ContextDef
	Apps             []AppDef
	Capabilities     []Capability
	CapabilityGroups []CapabilityGroup
	Roles            []Role
}

// ContextDef è un contesto dichiarato nell'ACL.
//
// HomeApp designa l'app home del contesto ed è il solo modo di dichiararla: senza designazione, il
// contesto ricade sull'app con BasePath "/" (vedi AppDef).
type ContextDef struct {
	ID          string
	Label       string
	Description string
	Icon        string
	Order       int
	HomeApp     string
}

// AppDef è un'app dichiarata nell'ACL. BasePath è il prefisso di rotta dell'app ("/" per la home,
// "/nome-app/" per le altre); l'Authorizer ci antepone il contesto quando ce n'è uno.
type AppDef struct {
	ID          string
	Description string
	BasePath    string
	Icon        string
	Order       int
}

// Capability è un permesso elementare. I campi significativi dipendono dalla Category:
//   - api:        APIPath + APIMethods (per MatchRequest), OperationID
//   - ui:         Endpoint, Icon, Order, Menu, AppID
//   - action_ui:  AppID
//   - action_api: solo l'id
//
// AppID vuoto significa "vale per ogni app".
type Capability struct {
	ID          string
	Category    string
	Description string
	OperationID string
	APIPath     string
	APIMethods  []string
	Endpoint    string
	Icon        string
	Order       int
	Menu        bool
	AppID       string
}

// CapabilityGroup raccoglie capability sotto un nome, perché un ruolo possa riferirle in blocco.
type CapabilityGroup struct {
	ID           string
	Description  string
	Capabilities []string
}

// Role è un ruolo dell'ACL. ContextID vuoto significa context-agnostic: il ruolo vale su ogni
// contesto, e dà accesso a tutti.
//
// CapabilityGroups e Capabilities sono riferimenti per id e NON sono risolti: li espande l'engine.
type Role struct {
	ID               string
	ContextID        string
	Description      string
	CapabilityGroups []string
	Capabilities     []string
}
