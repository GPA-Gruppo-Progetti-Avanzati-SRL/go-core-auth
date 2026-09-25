package apiauth

import (
	core "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	coreauth "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-auth"
)

// Module registra il middleware HTTP. Si passa a coreauth.WithMiddleware per riferimento diretto:
//
//	coreauth.Module(&svc.Auth, coreauth.WithSource(...), coreauth.WithMiddleware(apiauth.Module))
//
// Il *Middleware esce dal modulo perché è il router di go-core-api a montarlo.
func Module(modes ...string) {
	core.Provide(newMiddleware, modes...)
}

func newMiddleware(authorizer coreauth.Authorizer, cfg *coreauth.Config) *Middleware {
	return New(authorizer, cfg.Middleware)
}
