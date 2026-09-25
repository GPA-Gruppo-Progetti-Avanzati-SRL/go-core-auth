package mongosource

// Discriminatori del campo `_et`: l'ACL è una collection sola e il tipo di entità è un campo del
// documento. Un'unica collection permette di leggere l'ACL intero con una sola query, che è la
// ragione per cui lo Snapshot arriva coerente invece che assemblato da cinque letture
// indipendenti.
const (
	etContext         = "CONTEXT"
	etApp             = "APP"
	etRole            = "ROLE"
	etCapability      = "CAPABILITY"
	etCapabilityGroup = "CAPABILITYGROUP"
)

// aclDoc è un documento della collection ACL. I campi significativi dipendono da ET: tenerli in
// una struct sola è ciò che permette la lettura in un passaggio unico, e ogni campo è comunque
// omitempty, quindi un documento non porta le chiavi che non lo riguardano.
type aclDoc struct {
	ID string `bson:"_id"`
	ET string `bson:"_et"`

	Description string `bson:"description,omitempty"`
	Icon        string `bson:"icon,omitempty"`
	Order       int    `bson:"order,omitempty"`

	// CONTEXT
	Label   string `bson:"label,omitempty"`
	HomeApp string `bson:"home_app,omitempty"`

	// APP
	Path string `bson:"path,omitempty"`

	// ROLE
	ContextID        string   `bson:"_cid,omitempty"`
	CapabilityGroups []string `bson:"capability_groups,omitempty"`

	// ROLE, CAPABILITYGROUP
	Capabilities []string `bson:"capabilities,omitempty"`

	// CAPABILITY
	Category string   `bson:"category,omitempty"`
	AppID    string   `bson:"appId,omitempty"`
	API      *apiSpec `bson:"api,omitempty"`
	UI       *uiSpec  `bson:"ui,omitempty"`
}

// apiSpec descrive una capability di categoria "api".
//
// Path e Methods servono al controllo per rotta (MatchRequest); OperationID resta per gli ACL
// scritti sull'operationId di huma.
type apiSpec struct {
	OperationID string   `bson:"operationid,omitempty"`
	Path        string   `bson:"path,omitempty"`
	Methods     []string `bson:"methods,omitempty"`
}

// uiSpec descrive una capability di categoria "ui": la voce di menu.
type uiSpec struct {
	Endpoint string `bson:"endpoint,omitempty"`
	Icon     string `bson:"icon,omitempty"`
	Order    int    `bson:"order,omitempty"`
	Menu     bool   `bson:"menu,omitempty"`
}
