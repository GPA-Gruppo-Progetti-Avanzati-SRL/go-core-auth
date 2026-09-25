// Package coreauth è la libreria di autorizzazione di GPA: un solo engine, N sorgenti di ACL.
//
// La superficie che un'applicazione consuma è piccola di proposito: l'interfaccia Authorizer con i
// suoi DTO, la Config, e Module con le sue Option. Tutto il resto — l'engine, la view risolta, la
// sorgente — è un dettaglio del modulo.
//
// Una sorgente non implementa l'autorizzazione: legge l'ACL grezzo e restituisce uno Snapshot
// (vedi Source). È ciò che rende il costo di un backend nuovo pari alle sue query, invece che a
// una seconda implementazione del contratto.
package coreauth

// Authorizer espone il controllo e la consultazione delle autorizzazioni fra ruoli e funzioni.
//
// Tutti i metodi sono sicuri per uso concorrente e non bloccano mai su I/O: rispondono sulla view
// corrente e, se scaduta, ne fanno ricaricare una in background.
type Authorizer interface {
	// FilterRolesByContext filtra i ruoli per contesto: restituisce quelli associati a contextID
	// più quelli senza contesto (context-agnostic).
	//
	// Un contextID vuoto restituisce i ruoli invariati. Un risultato vuoto con contextID non vuoto
	// significa che l'utente non è autorizzato su quel contesto.
	FilterRolesByContext(roles []string, contextID string) []string

	// GetContexts restituisce i contesti accessibili ai ruoli passati. Va chiamato con i ruoli NON
	// filtrati: è la domanda che precede la scelta del contesto.
	//
	// Un ruolo context-agnostic dà accesso a tutti i contesti dichiarati nell'ACL.
	GetContexts(roles []string) []*Context

	// GetApps restituisce le app navigabili per i ruoli e il contesto indicati, con i path già
	// prefissati dal contesto. Va chiamato con i ruoli NON filtrati.
	GetApps(roles []string, contextID string) []*App

	// GetPaths restituisce le voci di menu (capability di categoria "ui") autorizzate per i ruoli
	// nell'app indicata. Le capability senza app-id valgono per ogni app.
	GetPaths(roles []string, appID string) []*Path

	// GetCapabilities restituisce gli id delle capability di categoria "action_ui" abilitate per i
	// ruoli nell'app indicata. Le capability senza app-id valgono per ogni app.
	GetCapabilities(roles []string, appID string) []string

	// GetServerCapabilities restituisce gli id delle capability di categoria "api" abilitate per i
	// ruoli. Non sono app-scoped: le usa il gateway per iniettare X-Capabilities verso i backend.
	GetServerCapabilities(roles []string) []string

	// HasCapability verifica se almeno uno dei ruoli possiede la capability "action_api" indicata.
	// È il controllo che la business logic di un'applicazione esegue su una singola azione.
	HasCapability(roles []string, capabilityID string) bool

	// MatchRequest verifica l'autorizzazione di una richiesta HTTP identificata da path e metodo,
	// sulle capability di categoria "api".
	//
	// Il path dichiarato nell'ACL può essere un glob: /api/persons/**, /api/persons/*,
	// /api/persons/:id, /api/persons/{id}. Una capability senza metodi vale per tutti i metodi.
	MatchRequest(roles []string, path, method string) bool

	// AllContextIDs restituisce gli id di tutti i contesti dell'ACL, indipendentemente dai ruoli.
	// Serve al gateway per registrare al boot le rotte /{cid}/*, quando nessun utente è noto.
	AllContextIDs() []string

	// HomeAppForContext restituisce l'id dell'app home designata per il contesto indicato, o la
	// stringa vuota se quel contesto non ne designa una.
	HomeAppForContext(contextID string) string
}

// Context è un contesto accessibile, come lo riceve chi interroga l'Authorizer.
type Context struct {
	ID          string `json:"id"`
	Description string `json:"description,omitempty"`
	Label       string `json:"label,omitempty"`
	Icon        string `json:"icon,omitempty"`
	Order       int    `json:"order,omitempty"`
}

// Path è una voce di menu autorizzata.
type Path struct {
	ID          string `json:"id"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
	Order       int    `json:"order,omitempty"`
	Menu        bool   `json:"ismenu"`
	Endpoint    string `json:"path,omitempty"`
}

// App è un'app navigabile, col path già risolto per il contesto corrente.
type App struct {
	ID          string `json:"id"`
	Description string `json:"description,omitempty"`
	Path        string `json:"path,omitempty"`
	Icon        string `json:"icon,omitempty"`
	Order       int    `json:"order,omitempty"`
	ContextID   string `json:"context_id,omitempty"`
}
