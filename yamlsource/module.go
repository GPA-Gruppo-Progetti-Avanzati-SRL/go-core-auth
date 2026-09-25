package yamlsource

import (
	core "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	coreauth "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-auth"
)

// Module registra la sorgente yaml. Si passa a coreauth.WithSource per riferimento diretto:
//
//	coreauth.Module(&svc.Auth, coreauth.WithSource(yamlsource.Module))
//
// Il Catalog è opzionale: senza, l'ACL è quello del solo file.
func Module(modes ...string) {
	core.Provide(newSource, modes...)
}

type params struct {
	core.In

	Config  *coreauth.Config
	Catalog Catalog `optional:"true"`
}

func newSource(p params) coreauth.Source {
	return &source{path: p.Config.Yaml.RulesFile, catalog: p.Catalog}
}
