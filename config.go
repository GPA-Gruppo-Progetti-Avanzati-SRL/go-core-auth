package coreauth

import "time"

// Default della libreria. Sono valori, non costanti di comodo: il refresh decide quanto a lungo
// una revoca resta invisibile, il timeout quanto a lungo un backend lento tiene occupata la
// goroutine di ricaricamento.
const (
	DefaultRefresh     = 10 * time.Minute
	DefaultLoadTimeout = 30 * time.Second
	DefaultCollection  = "acl"

	DefaultRolesHeader   = "X-Roles"
	DefaultContextHeader = "X-Context"
	DefaultUserHeader    = "X-User"
	DefaultDelimiter     = ","
)

// Config è la sezione `services.auth` della configurazione.
//
// Contiene anche le sezioni delle sorgenti: stanno qui e non nei rispettivi sottopackage perché il
// package root non deve importarli — è la condizione per cui un'applicazione che sceglie una sola
// sorgente non si porta dietro i driver delle altre.
type Config struct {
	// Refresh è l'età oltre la quale la view dell'ACL è considerata scaduta e il primo lettore ne
	// innesca il ricaricamento in background.
	Refresh time.Duration `yaml:"refresh" mapstructure:"refresh" json:"refresh"`
	// LoadTimeout limita il singolo caricamento dell'ACL.
	LoadTimeout time.Duration `yaml:"load-timeout" mapstructure:"load-timeout" json:"load-timeout"`
	// FailFastOnBoot ferma l'avvio se il primo caricamento fallisce.
	//
	// Il default è false: il processo parte con un ACL vuoto, cioè negando tutto, e si ricarica da
	// sé al primo lettore. È fail-closed — un backend momentaneamente irraggiungibile al boot non
	// deve impedire al processo di partire, e ciò che parte non concede nulla.
	FailFastOnBoot bool `yaml:"fail-fast-on-boot" mapstructure:"fail-fast-on-boot" json:"fail-fast-on-boot"`

	Mongo      MongoConfig       `yaml:"mongo"      mapstructure:"mongo"      json:"mongo"`
	Yaml       YAMLConfig        `yaml:"yaml"       mapstructure:"yaml"       json:"yaml"`
	Middleware *MiddlewareConfig `yaml:"middleware" mapstructure:"middleware" json:"middleware"`
}

// MongoConfig configura la sorgente mongo: il solo id della collection dell'ACL, risolto dal
// linked service di go-core-mongo.
type MongoConfig struct {
	Collection string `yaml:"collection" mapstructure:"collection" json:"collection"`
}

// La sorgente sql non ha configurazione: i nomi delle sue otto tabelle li fissa la libreria
// (acl_*), come go-core-sql/locker fa con scheduler_locks. Lo schema in cui vivono lo sceglie la
// connessione, che è dove quella scelta è già espressa.

// YAMLConfig configura la sorgente yaml: il file delle regole (capability-group e ruoli). App,
// contesti e capability scoperte al boot le passa il chiamante, perché non vengono da lì.
type YAMLConfig struct {
	RulesFile string `yaml:"rules-file" mapstructure:"rules-file" json:"rules-file"`
}

// MiddlewareConfig configura il middleware HTTP (sottopackage apiauth): da quali header arrivano
// identità, ruoli e contesto, e quali rotte sono esenti.
type MiddlewareConfig struct {
	Enabled       bool     `yaml:"enabled"        mapstructure:"enabled"        json:"enabled"`
	RolesHeader   string   `yaml:"roles-header"   mapstructure:"roles-header"   json:"roles-header"`
	ContextHeader string   `yaml:"context-header" mapstructure:"context-header" json:"context-header"`
	UserHeader    string   `yaml:"user-header"    mapstructure:"user-header"    json:"user-header"`
	Delimiter     string   `yaml:"delimiter"      mapstructure:"delimiter"      json:"delimiter"`
	GuestPaths    []string `yaml:"guest-paths"    mapstructure:"guest-paths"    json:"guest-paths"`
}

// WithDefaults riempie i campi non valorizzati. È idempotente.
func (c *Config) WithDefaults() *Config {
	if c.Refresh <= 0 {
		c.Refresh = DefaultRefresh
	}
	if c.LoadTimeout <= 0 {
		c.LoadTimeout = DefaultLoadTimeout
	}
	if c.Mongo.Collection == "" {
		c.Mongo.Collection = DefaultCollection
	}
	if c.Middleware != nil {
		c.Middleware.WithDefaults()
	}
	return c
}

// WithDefaults riempie gli header non valorizzati con la convenzione del gateway.
func (m *MiddlewareConfig) WithDefaults() *MiddlewareConfig {
	if m.RolesHeader == "" {
		m.RolesHeader = DefaultRolesHeader
	}
	if m.ContextHeader == "" {
		m.ContextHeader = DefaultContextHeader
	}
	if m.UserHeader == "" {
		m.UserHeader = DefaultUserHeader
	}
	if m.Delimiter == "" {
		m.Delimiter = DefaultDelimiter
	}
	return m
}
