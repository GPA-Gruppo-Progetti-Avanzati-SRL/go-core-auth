package apiauth

import (
	"context"

	coreauth "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-auth"
)

// Le chiavi del context sono di un tipo non esportato, non stringhe.
//
// Una chiave stringa è visibile a ogni package che condivide il context: "user" e "roles" sono
// nomi che qualcun altro può scrivere, e il valore che si rilegge non è necessariamente quello che
// si è scritto. Con un tipo privato la collisione non è improbabile: è impossibile.
//
// Chi legge questi valori usa gli accessor esportati sotto.
type ctxKey int

const (
	keyUser ctxKey = iota
	keyRoles
	keyAllRoles
	keyContextID
	keyAuthorizer
)

// UserFrom restituisce l'identità presentata dalla richiesta, o "" se il middleware non è montato.
func UserFrom(ctx context.Context) string { return stringFrom(ctx, keyUser) }

// ContextIDFrom restituisce il contesto selezionato dalla richiesta, o "".
func ContextIDFrom(ctx context.Context) string { return stringFrom(ctx, keyContextID) }

// RolesFrom restituisce i ruoli validi nel contesto corrente: sono quelli su cui vanno fatti i
// controlli di autorizzazione applicativi.
func RolesFrom(ctx context.Context) []string { return stringsFrom(ctx, keyRoles) }

// AllRolesFrom restituisce tutti i ruoli presentati, senza il filtro del contesto. Servono dove la
// domanda precede la scelta del contesto — quali contesti e quali app sono raggiungibili.
func AllRolesFrom(ctx context.Context) []string { return stringsFrom(ctx, keyAllRoles) }

// AuthorizerFrom restituisce l'Authorizer della richiesta, o nil se il middleware non è montato.
//
// Un'applicazione che deve verificare una capability nella business logic può iniettarsi
// direttamente coreauth.Authorizer: questo accessor serve dove si ha in mano solo il context.
func AuthorizerFrom(ctx context.Context) coreauth.Authorizer {
	if v := ctx.Value(keyAuthorizer); v != nil {
		if a, ok := v.(coreauth.Authorizer); ok {
			return a
		}
	}
	return nil
}

func stringFrom(ctx context.Context, k ctxKey) string {
	if v := ctx.Value(k); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func stringsFrom(ctx context.Context, k ctxKey) []string {
	if v := ctx.Value(k); v != nil {
		if s, ok := v.([]string); ok {
			return s
		}
	}
	return nil
}
