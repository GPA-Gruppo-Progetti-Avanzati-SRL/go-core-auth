// Package mongosource legge l'ACL da MongoDB: è il backend mongo di go-core-auth.
//
//	coreauth.Module(&svc.Auth, coreauth.WithSource(mongosource.Module))
//
// L'ACL vive in una collection sola (`acl` per default), col tipo di entità nel campo `_et`. La
// forma dei documenti è in docs/acl-seed.js, il validator in docs/acl-validation.js e gli indici
// consigliati in docs/acl-indexes.js.
package mongosource

import (
	"context"

	core "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	coreauth "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-auth"
	coremongo "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-mongo"
	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-mongo/mongoutil"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Ambit e codici degli errori di questo backend.
const (
	Ambit                  = "go-core-auth/mongosource"
	CodeCollectionNotFound = "AUTH-MONGO-COLL"
	CodeFind               = "AUTH-MONGO-FIND"
	CodeCursor             = "AUTH-MONGO-CUR"
)

// source legge l'ACL dalla collection configurata.
type source struct {
	svc        *coremongo.Service
	collection string
}

// Load legge l'intera collection ACL in un solo passaggio e la smista per `_et`.
//
// Una query sola e non una per entità: l'ACL è piccolo e la sua coerenza interna conta — ruoli che
// nominano capability lette in un secondo momento sarebbero uno snapshot che non è mai esistito.
func (s *source) Load(ctx context.Context) (*coreauth.Snapshot, *core.ApplicationError) {
	coll := s.svc.GetCollection(s.collection, "")
	if coll == nil {
		return nil, core.TechnicalError().WithAmbit(Ambit).WithCode(CodeCollectionNotFound).
			WithMessage("collection '" + s.collection + "' non configurata nel linked service mongo")
	}

	cur, err := coll.Find(ctx, bson.M{})
	if err != nil {
		return nil, core.TechnicalError().WithAmbit(Ambit).WithCode(CodeFind).WithCause(err)
	}
	defer mongoutil.CloseCursor(ctx, cur, "mongosource.Load/acl")

	var docs []aclDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, core.TechnicalError().WithAmbit(Ambit).WithCode(CodeCursor).WithCause(err)
	}

	return toSnapshot(docs), nil
}

// toSnapshot smista i documenti nelle entità dello Snapshot.
func toSnapshot(docs []aclDoc) *coreauth.Snapshot {
	snap := &coreauth.Snapshot{}
	var unknown []string

	for _, d := range docs {
		if d.ID == "" {
			continue
		}
		switch d.ET {
		case etContext:
			snap.Contexts = append(snap.Contexts, coreauth.ContextDef{
				ID: d.ID, Label: d.Label, Description: d.Description,
				Icon: d.Icon, Order: d.Order, HomeApp: d.HomeApp,
			})
		case etApp:
			snap.Apps = append(snap.Apps, coreauth.AppDef{
				ID: d.ID, Description: d.Description, BasePath: d.Path,
				Icon: d.Icon, Order: d.Order,
			})
		case etRole:
			snap.Roles = append(snap.Roles, coreauth.Role{
				ID: d.ID, ContextID: d.ContextID, Description: d.Description,
				CapabilityGroups: d.CapabilityGroups, Capabilities: d.Capabilities,
			})
		case etCapabilityGroup:
			snap.CapabilityGroups = append(snap.CapabilityGroups, coreauth.CapabilityGroup{
				ID: d.ID, Description: d.Description, Capabilities: d.Capabilities,
			})
		case etCapability:
			snap.Capabilities = append(snap.Capabilities, toCapability(d))
		default:
			unknown = append(unknown, d.ID)
		}
	}

	if len(unknown) > 0 {
		// La collection è dedicata all'ACL: un `_et` che non si riconosce è quasi sempre un refuso
		// nel seed, e il suo effetto — una capability che non esiste per nessuno — non si distingue
		// da un permesso mai concesso.
		log.Warn().Int("count", len(unknown)).Strs("sample", first(unknown, 10)).
			Msg("ACL mongo: documenti con _et non riconosciuto, ignorati")
	}
	return snap
}

func toCapability(d aclDoc) coreauth.Capability {
	c := coreauth.Capability{
		ID:          d.ID,
		Category:    d.Category,
		Description: d.Description,
		AppID:       d.AppID,
		Icon:        d.Icon,
		Order:       d.Order,
	}
	if d.API != nil {
		c.OperationID = d.API.OperationID
		c.APIPath = d.API.Path
		c.APIMethods = d.API.Methods
	}
	if d.UI != nil {
		c.Endpoint = d.UI.Endpoint
		c.Menu = d.UI.Menu
		if d.UI.Icon != "" {
			c.Icon = d.UI.Icon
		}
		if d.UI.Order != 0 {
			c.Order = d.UI.Order
		}
	}
	return c
}

func first(s []string, n int) []string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
