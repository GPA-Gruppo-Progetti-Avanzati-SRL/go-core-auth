package apiauth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	coreapi "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-api"
	core "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	coreauth "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-auth"
	"github.com/danielgtaylor/huma/v2"
)

// Seed delle capability per i backend di go-core-auth.
//
// La serializzazione sta qui e non in go-core-api perché lo schema è di questa libreria: i nomi
// delle tabelle acl_* e la forma dei documenti della collection acl li stabiliscono sqlsource e
// mongosource, e un generatore che li scrive da un altro modulo è una seconda dichiarazione dello
// stesso schema. Di go-core-api resta ciò che solo lui può sapere — quali capability
// l'applicazione espone — che arriva da coreapi.Capabilities.
//
// Gli endpoint sono montati da Register in develop-mode, come gli altri di diagnostica:
//
//	GET /acl.coreauth.sql   → le tabelle acl_* di sqlsource
//	GET /acl.coreauth.js    → la collection acl di mongosource
//
// Nessuno dei due assegna capability a un ruolo: creano le capability e il gruppo che le raccoglie.
// Assegnare quel gruppo a un ruolo resta un atto esplicito di chi governa l'ACL — ed è la ragione
// per cui eseguire il seed non concede permessi a nessuno.

const (
	// SeedSQLPath e SeedMongoPath sono le rotte dei due seed, montate solo in develop-mode.
	SeedSQLPath   = "/acl.coreauth.sql"
	SeedMongoPath = "/acl.coreauth.js"
)

// SeedSQL rende lo script di upsert per le tabelle acl_* lette da go-core-auth/sqlsource.
// Idempotente, sintassi PostgreSQL / SQLite.
func SeedSQL(api huma.API) string {
	appID := core.AppName
	entries := coreapi.Capabilities(api)
	var sb strings.Builder

	sb.WriteString("-- Capability di " + appID + " per go-core-auth/sqlsource.\n")
	sb.WriteString("-- Generato da GET " + SeedSQLPath + " — idempotente, sintassi PostgreSQL / SQLite.\n")
	sb.WriteString("-- Non assegna il gruppo ad alcun ruolo: quella resta una decisione esplicita.\n\n")

	for _, e := range entries {
		fmt.Fprintf(&sb,
			"INSERT INTO acl_capability (id, category, description, operation_id, api_path, api_methods,\n"+
				"                            endpoint, icon, ord, menu, app_id)\n"+
				"VALUES (%s, %s, %s, %s, %s, %s, '', '', 0, FALSE, %s)\n"+
				"ON CONFLICT (id) DO UPDATE SET\n"+
				"    category = EXCLUDED.category, description = EXCLUDED.description,\n"+
				"    operation_id = EXCLUDED.operation_id, api_path = EXCLUDED.api_path,\n"+
				"    api_methods = EXCLUDED.api_methods, app_id = EXCLUDED.app_id;\n\n",
			sqlStr(coreapi.CapabilityID(appID, e.ID)), sqlStr(e.Category), sqlStr(descriptionOf(e)),
			sqlStr(operationIDOf(e)), sqlStr(e.Endpoint), sqlStr(e.Method), sqlStr(appID),
		)
	}

	groupID := seedGroupID(appID)
	fmt.Fprintf(&sb,
		"INSERT INTO acl_capability_group (id, description)\n"+
			"VALUES (%s, %s)\n"+
			"ON CONFLICT (id) DO UPDATE SET description = EXCLUDED.description;\n\n",
		sqlStr(groupID), sqlStr("Tutte le capability di "+appID),
	)
	for _, e := range entries {
		fmt.Fprintf(&sb,
			"INSERT INTO acl_capability_group_item (group_id, capability_id) VALUES (%s, %s)\n"+
				"ON CONFLICT (group_id, capability_id) DO NOTHING;\n",
			sqlStr(groupID), sqlStr(coreapi.CapabilityID(appID, e.ID)),
		)
	}
	return sb.String()
}

// SeedMongo rende lo script di upsert per la collection acl letta da go-core-auth/mongosource.
func SeedMongo(api huma.API) string {
	appID := core.AppName
	entries := coreapi.Capabilities(api)
	var sb strings.Builder

	sb.WriteString("// Capability di " + appID + " per go-core-auth/mongosource.\n")
	sb.WriteString("// Generato da GET " + SeedMongoPath + " — idempotente.\n")
	sb.WriteString("// Non assegna il gruppo ad alcun ruolo: quella resta una decisione esplicita.\n\n")
	sb.WriteString("const COLLECTION = \"acl\";\n\n")

	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		id := coreapi.CapabilityID(appID, e.ID)
		ids = append(ids, id)

		doc := seedCapDoc{
			ID: id, ET: "CAPABILITY", Category: e.Category,
			Description: descriptionOf(e), AppID: appID,
		}
		// Il sottodocumento api riguarda le sole capability che autorizzano una rotta: su una
		// action_api sarebbe un campo che nessuno legge, e il validator della collection lo rifiuta.
		if e.Category == coreauth.CategoryAPI {
			doc.API = &seedCapAPIDoc{OperationID: operationIDOf(e), Path: e.Endpoint}
			if e.Method != "" {
				doc.API.Methods = []string{e.Method}
			}
		}
		writeUpsert(&sb, id, doc)
	}

	groupID := seedGroupID(appID)
	writeUpsert(&sb, groupID, struct {
		ID           string   `json:"_id"`
		ET           string   `json:"_et"`
		Description  string   `json:"description"`
		Capabilities []string `json:"capabilities"`
	}{groupID, "CAPABILITYGROUP", "Tutte le capability di " + appID, ids})

	return sb.String()
}

type seedCapDoc struct {
	ID          string         `json:"_id"`
	ET          string         `json:"_et"`
	Category    string         `json:"category"`
	Description string         `json:"description,omitempty"`
	AppID       string         `json:"appId,omitempty"`
	API         *seedCapAPIDoc `json:"api,omitempty"`
}

type seedCapAPIDoc struct {
	OperationID string   `json:"operationid,omitempty"`
	Path        string   `json:"path,omitempty"`
	Methods     []string `json:"methods,omitempty"`
}

// seedGroupID è il gruppo che raccoglie tutte le capability dell'applicazione.
func seedGroupID(appID string) string { return fmt.Sprintf("grp:%s:ALL", appID) }

func writeUpsert(sb *strings.Builder, id string, doc any) {
	raw, _ := json.MarshalIndent(doc, "    ", "    ")
	fmt.Fprintf(sb,
		"db.getCollection(COLLECTION).replaceOne(\n    { _id: %s },\n    %s,\n    { upsert: true }\n)\n\n",
		jsonStr(id), string(raw),
	)
}

// descriptionOf: una capability senza descrizione resta comunque riconoscibile dal suo id.
func descriptionOf(e coreapi.CapabilityEntry) string {
	if e.Description != "" {
		return e.Description
	}
	return e.ID
}

// operationIDOf restituisce l'operationId di huma. coreapi.Capabilities lo omette quando coincide
// con l'id della capability, ma nel seed va scritto comunque: è il campo con cui un ACL basato
// sull'operationId trova la capability.
func operationIDOf(e coreapi.CapabilityEntry) string {
	if e.OperationID != "" {
		return e.OperationID
	}
	return e.ID
}

// sqlStr racchiude una stringa in apici singoli raddoppiando quelli interni (SQL standard).
func sqlStr(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// mountSeed registra i due endpoint di seed sul mux del router.
func mountSeed(r *coreapi.Router) {
	r.Mux.Get(SeedSQLPath, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(SeedSQL(r.Api)))
	})
	r.Mux.Get(SeedMongoPath, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(SeedMongo(r.Api)))
	})
}
