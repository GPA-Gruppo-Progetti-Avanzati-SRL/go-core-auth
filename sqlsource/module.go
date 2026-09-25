package sqlsource

import (
	core "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	coreauth "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-auth"
	coresql "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-sql"
)

// Module registra la sorgente sql. Si passa a coreauth.WithSource per riferimento diretto:
//
//	coreauth.Module(&svc.Auth, coreauth.WithSource(sqlsource.Module))
//
// Richiede il *coresql.Service dell'applicazione, cioè che coresql.Module sia wirato.
func Module(modes ...string) {
	core.ProvideAs[coreauth.Source](newSource, modes...)
}

func newSource(svc *coresql.Service) coreauth.Source {
	return &source{db: svc.IDB()}
}
