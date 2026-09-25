package mongosource

import (
	core "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	coreauth "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-auth"
	coremongo "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-mongo"
)

// Module registra la sorgente mongo. Si passa a coreauth.WithSource per riferimento diretto:
//
//	coreauth.Module(&svc.Auth, coreauth.WithSource(mongosource.Module))
//
// Richiede il *coremongo.Service dell'applicazione, cioè che coremongo.Module sia wirato.
func Module(modes ...string) {
	core.ProvideAs[coreauth.Source](newSource, modes...)
}

func newSource(svc *coremongo.Service, cfg *coreauth.Config) coreauth.Source {
	collection := cfg.Mongo.Collection
	if collection == "" {
		collection = coreauth.DefaultCollection
	}
	return &source{svc: svc, collection: collection}
}
