package yamlsource

// Le struct del file delle regole. Rispecchiano il modello degli altri backend: contesti, app,
// capability, gruppi e ruoli — così lo stesso ACL si scrive in YAML, su Mongo o su SQL senza
// tradurne il significato.

type rulesFile struct {
	Contexts         []contextRule    `yaml:"contexts"`
	Apps             []appRule        `yaml:"apps"`
	Capabilities     []capabilityRule `yaml:"capabilities"`
	CapabilityGroups []groupRule      `yaml:"capability-groups"`
	Roles            []roleRule       `yaml:"roles"`
}

type contextRule struct {
	ID          string `yaml:"id"`
	Label       string `yaml:"label"`
	Description string `yaml:"description"`
	Icon        string `yaml:"icon"`
	Order       int    `yaml:"order"`
	// HomeApp designa l'app home del contesto.
	HomeApp string `yaml:"home-app"`
}

type appRule struct {
	ID          string `yaml:"id"`
	Description string `yaml:"description"`
	BasePath    string `yaml:"base-path"`
	Icon        string `yaml:"icon"`
	Order       int    `yaml:"order"`
}

type capabilityRule struct {
	ID          string   `yaml:"id"`
	Category    string   `yaml:"category"`
	Description string   `yaml:"description"`
	AppID       string   `yaml:"app-id"`
	API         *apiRule `yaml:"api"`
	UI          *uiRule  `yaml:"ui"`
}

type apiRule struct {
	OperationID string   `yaml:"operation-id"`
	Path        string   `yaml:"path"`
	Methods     []string `yaml:"methods"`
}

type uiRule struct {
	Endpoint string `yaml:"endpoint"`
	Icon     string `yaml:"icon"`
	Order    int    `yaml:"order"`
	Menu     bool   `yaml:"menu"`
}

type groupRule struct {
	ID           string   `yaml:"id"`
	Description  string   `yaml:"description"`
	Capabilities []string `yaml:"capabilities"`
}

type roleRule struct {
	ID          string `yaml:"id"`
	Description string `yaml:"description"`
	// Context vuoto = ruolo context-agnostic.
	Context          string   `yaml:"context"`
	CapabilityGroups []string `yaml:"capability-groups"`
	Capabilities     []string `yaml:"capabilities"`
}
