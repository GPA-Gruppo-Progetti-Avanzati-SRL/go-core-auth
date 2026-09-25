# go-core-auth

Autorizzazione per i microservizi GPA: **un solo engine, N sorgenti di ACL**.

Una sorgente non implementa l'autorizzazione — legge l'ACL grezzo e restituisce uno `Snapshot`.
Tutto il resto (espansione dei capability-group, filtro per contesto, matching delle rotte,
ricaricamento) è dell'engine, ed è scritto una volta sola. Aggiungere un backend costa le sue
query, non una seconda implementazione del contratto.

```
      mongosource ─┐
      sqlsource  ──┼─→ Snapshot ──→ engine ──→ Authorizer ──→ apiauth (middleware HTTP + /api/token)
      yamlsource ──┘                                      └──→ business logic dell'app
```

## Wiring

```go
coreauth.Module(&svc.Auth,
    coreauth.WithSource(mongosource.Module),   // obbligatoria
    coreauth.WithMiddleware(apiauth.Module),   // solo per chi espone un'API
    coreauth.WithModes(engine.Api))
```

`Module` espone al grafo fx **l'`Authorizer` e nient'altro**: `Config`, `Source` ed engine restano
dentro il modulo. Scambiare backend è scambiare un import — `sqlsource.Module`,
`yamlsource.Module` — ed è anche ciò che decide quali dipendenze finiscono nel `go.mod`
dell'applicazione: un'app mongo non si porta dietro bun, una senza API non si porta huma.

`WithSource` è obbligatoria e la sua assenza è un panic al boot: senza sorgente l'ACL è vuoto e
l'autorizzazione nega tutto, cioè il servizio parte e non funziona.

## Config — `services.auth`

```yaml
config:
  services:
    auth:
      refresh: 10m               # età oltre la quale l'ACL viene ricaricato
      load-timeout: 30s          # limite del singolo caricamento
      fail-fast-on-boot: false   # true = l'avvio fallisce se il primo caricamento non riesce
      mongo:
        collection: acl          # id della collection nel linked service mongo
      yaml:
        rules-file: /etc/acl/auth-rules.yml
      middleware:
        enabled: true
        roles-header: X-Roles
        context-header: X-Context
        user-header: X-User
        delimiter: ","
        guest-paths: []          # rotte huma esenti dall'autorizzazione
```

La sorgente sql non ha configurazione: i nomi delle sue tabelle li fissa la libreria (`acl_*`),
come `go-core-sql/locker` fa con `scheduler_locks`. Lo schema in cui vivono lo sceglie la
connessione, che è dove quella scelta è già espressa.

## Il modello ACL

Cinque entità, le stesse in tutti e tre i backend:

| Entità | A cosa serve |
|---|---|
| **Context** | Un ambito di lavoro (area, società, tenant). `home-app` designa l'app home del contesto. |
| **App** | Un'applicazione navigabile. `base-path` è il suo prefisso di rotta; con un contesto selezionato l'Authorizer vi antepone `/{contesto}`. |
| **Capability** | Un permesso elementare. La `category` decide chi lo legge: `api` autorizza una richiesta HTTP, `ui` è una voce di menu, `action_ui` un comando dell'interfaccia, `action_api` un'azione della business logic. Senza `app-id` vale per ogni app. |
| **CapabilityGroup** | Capability raccolte sotto un nome, perché un ruolo le riferisca in blocco. |
| **Role** | Ciò che la richiesta presenta. Un ruolo senza contesto è *context-agnostic*: vale ovunque e dà accesso a tutti i contesti. |

Gli id di contesto sono **normalizzati** all'ingresso: `NORD`, `nord` e `Nord` sono lo stesso
contesto, comunque arrivino — dall'ACL, dall'header `X-Context` o da una rotta `/{cid}/`.

## L'interfaccia

```go
type Authorizer interface {
    FilterRolesByContext(roles []string, contextID string) []string
    GetContexts(roles []string) []*Context
    GetApps(roles []string, contextID string) []*App
    GetPaths(roles []string, appID string) []*Path
    GetCapabilities(roles []string, appID string) []string
    GetServerCapabilities(roles []string) []string
    HasCapability(roles []string, capabilityID string) bool
    MatchRequest(roles []string, path, method string) bool
    AllContextIDs() []string
    HomeAppForContext(contextID string) string
}
```

Nessun metodo blocca su I/O: rispondono sulla view corrente e, se scaduta, ne fanno ricaricare una
in background. `MatchRequest` accetta glob nel path dichiarato dall'ACL: `/api/persons/**`,
`/api/persons/*`, `/api/persons/:id`, `/api/persons/{id}`.

## Ricaricamento

Il refresh è **lazy-on-read**: lo innesca il primo lettore che trova la view scaduta, e quel
lettore risponde con la view che ha. Un processo senza traffico non interroga il backend per
mantenere aggiornato qualcosa che nessuno sta leggendo.

La view è **immutabile e sostituita in blocco**. Sono due proprietà, entrambe necessarie:

- una lettura concorrente vede tutto il vecchio o tutto il nuovo, mai un misto;
- **ciò che sparisce dall'ACL sparisce davvero.** Uno stato aggiornato chiave per chiave
  conserverebbe un ruolo revocato finché il processo non riparte — cioè una revoca che non revoca.

Un caricamento fallito **non svuota nulla**: resta in uso lo snapshot precedente e il log dice che
le decisioni proseguono su dati vecchi. Il fallimento è quindi sempre conservativo, in entrambe le
direzioni: al boot si nega tutto, a regime si continua con l'ultimo ACL valido.

## Middleware HTTP (`apiauth`)

`go-core-api` monta ciò che questo package fornisce; il montaggio resta suo perché la `huma.API` la
possiede lui. Il middleware riconosce identità, ruoli e contesto dagli header, riduce i ruoli al
contesto presentato e autorizza la rotta. Se non c'è nulla da montare non monta nulla, e non è un
errore.

I valori finiscono nel context della richiesta sotto chiavi di **tipo privato**, non stringhe, e si
leggono con gli accessor:

```go
apiauth.UserFrom(ctx)        // identità
apiauth.RolesFrom(ctx)       // ruoli validi nel contesto corrente ← per i controlli applicativi
apiauth.AllRolesFrom(ctx)    // tutti i ruoli, senza filtro di contesto
apiauth.ContextIDFrom(ctx)   // contesto selezionato
apiauth.AuthorizerFrom(ctx)  // l'Authorizer, per chi ha in mano solo il context
```

Con una chiave stringa, `"user"` e `"roles"` sono nomi che qualunque altro package può scrivere nel
medesimo context: il valore riletto non sarebbe necessariamente quello scritto.

**Senza Authorizer il middleware nega.** È l'unico caso in cui un guasto della configurazione
allargherebbe i permessi invece di restringerli, e un'API che lascia passare tutto non si distingue
da un'API che non protegge nulla.

### `/api/token`

Restituisce identità e permessi dell'utente, cifrati con l'`AppId` (header obbligatorio) come
chiave. Le tre domande non usano gli stessi ruoli, ed è voluto: le **app navigabili** si calcolano
su *tutti* i ruoli, perché "dove posso andare" precede la scelta del contesto; **menu e comandi**
sui ruoli del contesto corrente, perché descrivono ciò che si può fare qui e ora.

## I backend

| Package | Sorgente | Dipendenza |
|---|---|---|
| `mongosource` | collection `acl`, tipo di entità nel campo `_et` | `go-core-mongo` |
| `sqlsource` | otto tabelle `acl_*` | `go-core-sql` (bun) |
| `yamlsource` | un file YAML, riletto a ogni ricaricamento | `go.yaml.in/yaml/v3` |

**mongo** — un'unica `find({})`: l'ACL è piccolo e la sua coerenza interna conta più del numero di
documenti letti. Il percorso di lettura non ha bisogno di indici. Seed, validator e indici di
amministrazione sono in `mongosource/docs/`.

**sql** — le otto tabelle lette in **una transazione di sola lettura**: non per scrivere, ma per
leggere uno stato solo. Senza, le associazioni potrebbero essere lette dopo una modifica che le
entità non hanno visto, e lo Snapshot descriverebbe un ACL mai esistito. `EnsureTables` crea le
tabelle ed è esplicita: creare tabelle è una modifica allo schema, e dove le migrazioni sono
governate deve restare una scelta di chi le governa.

**yaml** — il file contiene l'ACL intero e viene riletto a ogni caricamento: modificarlo e
attendere l'intervallo di refresh basta a cambiare i permessi, senza riavviare il processo. Un
`Catalog` supplito dall'applicazione aggiunge le entità che il file non può conoscere — le
capability scoperte al boot dai manifest dei frontend. In conflitto sullo stesso id vince il file,
che è la dichiarazione esplicita.

## Seed delle capability

Le capability di un microservizio le genera `go-core-api` dagli endpoint registrati, in
develop-mode:

| Endpoint | Destinatario |
|---|---|
| `GET /acl.coreauth.sql` | le tabelle `acl_*` di `sqlsource` |
| `GET /acl.coreauth.js` | la collection `acl` di `mongosource` |
| `GET /acl.sql`, `GET /acl.mongo.js` | il **frontdoor OPEM** (`opem_acl_*`, `_et: cap-def`) — altro schema, altro consumatore |
| `GET /capabilities`, `/capabilities.yaml` | il resto della catena (discovery del gateway) |

I primi due sono distinti dagli altri perché i due mondi hanno schemi diversi e destinatari
diversi: un generatore solo produrrebbe per l'uno un seed che l'altro non sa leggere.

Lo script **non assegna nulla ad alcun ruolo**: crea le capability e il gruppo che le raccoglie.
Assegnare il gruppo a un ruolo resta un atto esplicito di chi governa l'ACL — ed è la ragione per
cui eseguirlo non concede permessi a nessuno.
