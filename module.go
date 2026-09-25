package coreauth

import (
	"context"

	core "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	"github.com/rs/zerolog/log"
	"go.uber.org/fx"
)

// ModuleFunc è la forma con cui si passa un backend: una registrazione fx che riceve solo i modes.
//
// Si passa per riferimento diretto — coreauth.WithSource(mongosource.Module) — e mai come closure:
// è ciò che fa sì che il go.mod dell'applicazione elenchi soltanto il backend che ha davvero
// importato, invece di tutti quelli che la libreria conosce.
type ModuleFunc func(modes ...string)

// Option configura Module.
type Option func(*options)

type options struct {
	modes      []string
	source     ModuleFunc
	middleware ModuleFunc
}

// WithModes limita la registrazione ai core.Mode indicati (nessuno = sempre).
func WithModes(modes ...string) Option {
	return func(o *options) { o.modes = modes }
}

// WithSource sceglie il backend da cui leggere l'ACL. È obbligatoria.
//
//	coreauth.WithSource(mongosource.Module)   // go-core-auth/mongosource
//	coreauth.WithSource(sqlsource.Module)     // go-core-auth/sqlsource
//	coreauth.WithSource(yamlsource.Module)    // go-core-auth/yamlsource
func WithSource(m ModuleFunc) Option {
	return func(o *options) { o.source = m }
}

// WithMiddleware registra il middleware HTTP e l'endpoint del token di sessione, che go-core-api
// monta sul proprio router.
//
// È un'Option e non un default perché è l'unica parte della libreria che dipende da huma: solo
// l'applicazione che espone un'API la importa, e solo lei se la porta nel proprio grafo.
//
//	coreauth.WithMiddleware(apiauth.Module)   // go-core-auth/apiauth
func WithMiddleware(m ModuleFunc) Option {
	return func(o *options) { o.middleware = m }
}

// Module wira l'autorizzazione: è l'unico entry-point della libreria.
//
//	coreauth.Module(&svc.Auth,
//	    coreauth.WithSource(mongosource.Module),
//	    coreauth.WithMiddleware(apiauth.Module),
//	    coreauth.WithModes(engine.Api))
//
// Espone al grafo l'Authorizer e nient'altro: Config, Source ed engine restano dentro il modulo.
func Module(cfg *Config, opts ...Option) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if o.source == nil {
		panic("coreauth.Module: WithSource è obbligatoria — senza una sorgente l'ACL è vuoto e " +
			"l'autorizzazione nega tutto. Scegliere fra coreauth.WithSource(mongosource.Module), " +
			"(sqlsource.Module) o (yamlsource.Module)")
	}
	cfg.WithDefaults()

	core.Module("auth", func() {
		core.Supply(cfg, o.modes...)

		// Il Source è l'ingranaggio dell'engine, non un servizio dell'applicazione: resta privato
		// al modulo. Sta qui dentro e non fuori come lo store di batch proprio perché l'engine
		// dipende da lui — un costruttore registrato a root non vedrebbe un provider privato di un
		// modulo discendente.
		core.Private(func() { o.source(o.modes...) })

		core.ProvideAs[Authorizer](newAuthorizer, o.modes...)

		if o.middleware != nil {
			o.middleware(o.modes...)
		}
	})
}

// newAuthorizer costruisce l'engine e ne programma il primo caricamento all'avvio.
func newAuthorizer(lc fx.Lifecycle, cfg *Config, src Source) Authorizer {
	l := newLUT(src, cfg.Refresh, cfg.LoadTimeout)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Info().Dur("refresh", cfg.Refresh).Dur("load-timeout", cfg.LoadTimeout).
				Bool("fail-fast-on-boot", cfg.FailFastOnBoot).Msg("autorizzazione: caricamento ACL")

			err := l.load(ctx)
			if err == nil {
				return nil
			}
			if cfg.FailFastOnBoot {
				return err.WithCode(CodeBootLoad)
			}
			// L'avvio prosegue, ma va detto cosa significa: finché un ricaricamento non riesce,
			// l'ACL è vuoto e nessuna richiesta è autorizzata. È il momento peggiore in cui non
			// lasciare traccia.
			log.Error().Err(err).
				Msg("autorizzazione: caricamento iniziale dell'ACL fallito, si parte con ACL vuoto " +
					"(nessuna autorizzazione concessa) finché un ricaricamento non riesce")
			return nil
		},
	})

	return l
}
