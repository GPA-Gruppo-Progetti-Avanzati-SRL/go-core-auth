# Errori di go-core-auth

Ogni errore nato dentro questa libreria porta un `Ambit` che nomina il package che l'ha prodotto —
non l'`AppName`, che i costruttori di `core.*Error()` riempiono da soli e che indicherebbe
l'applicazione che *riceve* l'errore invece della libreria che l'ha generato.

## Engine e middleware — ambit `go-core-auth`

| Codice | Costante | Origine | Significato |
|---|---|---|---|
| `AUTH-SRC-LOAD` | `coreauth.CodeSourceLoad` | `engine.go` | La sorgente ha restituito uno Snapshot nullo senza segnalare un errore. Resta in uso lo snapshot precedente. |
| `AUTH-BOOT-LOAD` | `coreauth.CodeBootLoad` | `module.go` | Il primo caricamento dell'ACL è fallito con `fail-fast-on-boot: true`: l'avvio si ferma. Col default (`false`) l'errore è solo loggato e il processo parte con ACL vuoto, cioè negando tutto. |
| `AUTH-FORBIDDEN` | `coreauth.CodeForbiddenRole` | `apiauth/middleware.go` | 403: nessun ruolo presentato abilita la rotta, oppure la richiesta non porta ruoli, oppure non c'è un Authorizer con cui decidere. |
| `AUTH-CTX-FORBIDDEN` | `coreauth.CodeForbiddenCtx` | `apiauth/middleware.go` | 403: il contesto presentato non è autorizzato per quei ruoli. |
| `AUTH-TOKEN-CRYPT` | `coreauth.CodeTokenEncryption` | `apiauth/token.go` | Serializzazione o cifratura del token di sessione fallite. |

Il corpo dei 403 ha la stessa forma delle altre risposte d'errore — `{"ambit","code","message"}` —
perché un rifiuto di autorizzazione è un errore applicativo come gli altri e un client non deve
avere un ramo apposta per trattarlo.

## Sorgente mongo — ambit `go-core-auth/mongosource`

| Codice | Costante | Significato |
|---|---|---|
| `AUTH-MONGO-COLL` | `mongosource.CodeCollectionNotFound` | La collection dell'ACL non è configurata nel linked service mongo. |
| `AUTH-MONGO-FIND` | `mongosource.CodeFind` | La lettura della collection è fallita. |
| `AUTH-MONGO-CUR` | `mongosource.CodeCursor` | La decodifica dei documenti è fallita. |

## Sorgente sql — ambit `go-core-auth/sqlsource`

| Codice | Costante | Significato |
|---|---|---|
| `AUTH-SQL-SELECT` | `sqlsource.CodeSelect` | La lettura delle tabelle dell'ACL è fallita. |
| `AUTH-SQL-DDL` | `sqlsource.CodeEnsureDDL` | `EnsureTables` non è riuscita a creare una tabella. |

## Sorgente yaml — ambit `go-core-auth/yamlsource`

| Codice | Costante | Significato |
|---|---|---|
| `AUTH-YAML-NOFILE` | `yamlsource.CodeNoRules` | `services.auth.yaml.rules-file` non è valorizzato. |
| `AUTH-YAML-READ` | `yamlsource.CodeReadRules` | Il file delle regole non è leggibile. |
| `AUTH-YAML-PARSE` | `yamlsource.CodeParseRules` | Il file delle regole non è YAML valido. |

## Quello che non è un errore

Due situazioni si segnalano con un log e non con un errore, ed è deliberato.

**Un riferimento pendente** — un ruolo che nomina un gruppo o una capability inesistenti — non
invalida l'ACL: il resto resta utilizzabile. Ma è un permesso che non arriverà mai a destinazione,
e il suo sintomo è un 403 che nessuna configurazione spiega: l'engine lo conta e ne logga un
campione a `Warn` a ogni caricamento.

**Un documento con `_et` non riconosciuto** in Mongo viene ignorato con un `Warn`. La collection è
dedicata all'ACL, quindi è quasi sempre un refuso nel seed, e il suo effetto — una capability che
non esiste per nessuno — non si distingue da un permesso mai concesso.
