package apiauth

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"

	core "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	coreauth "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-auth"
	"github.com/danielgtaylor/huma/v2"
)

// TokenPath è la rotta dell'endpoint del token di sessione.
const TokenPath = "/api/token"

// tokenInput: l'app di destinazione arriva come header, perché menu e comandi sono per app.
type tokenInput struct {
	AppID string `header:"AppId" required:"true"`
}

// RawStringOutput porta un corpo già serializzato: il token è testo cifrato, non un oggetto JSON
// da rimarshallare.
type RawStringOutput struct {
	ContentType string `header:"Content-Type"`
	Body        []byte
}

func (r *RawStringOutput) MarshalJSON() ([]byte, error) { return r.Body, nil }

type tokenBody struct {
	User         string           `json:"user"`
	Context      string           `json:"context,omitempty"`
	Roles        []string         `json:"roles"`
	Capabilities []string         `json:"capabilities"`
	Apps         []*coreauth.App  `json:"apps"`
	Paths        []*coreauth.Path `json:"paths"`
}

func tokenOperation() huma.Operation {
	return huma.Operation{
		OperationID:   "Token",
		Method:        http.MethodGet,
		Path:          TokenPath,
		Summary:       "Identità dell'utente corrente e permessi derivati dai suoi ruoli",
		Tags:          []string{"system"},
		DefaultStatus: http.StatusOK,
		Responses: map[string]*huma.Response{
			"200": {
				Description: "Token cifrato",
				Content: map[string]*huma.MediaType{
					"text/plain": {Schema: &huma.Schema{Type: huma.TypeString}},
				},
			},
		},
	}
}

// token risponde con identità e permessi dell'utente, cifrati con l'AppID come chiave.
//
// Le tre domande non usano gli stessi ruoli, ed è voluto: le app navigabili si calcolano su TUTTI
// i ruoli, perché la domanda "dove posso andare" precede la scelta del contesto; menu e comandi si
// calcolano sui ruoli del contesto corrente, perché descrivono ciò che si può fare qui e ora.
func (m *Middleware) token(ctx context.Context, in *tokenInput) (*RawStringOutput, error) {
	contextRoles := RolesFrom(ctx)
	allRoles := AllRolesFrom(ctx)
	if len(allRoles) == 0 {
		allRoles = contextRoles
	}
	contextID := ContextIDFrom(ctx)

	body := &tokenBody{
		User:    UserFrom(ctx),
		Context: contextID,
		Roles:   contextRoles,
	}
	if m.authorizer != nil {
		body.Apps = m.authorizer.GetApps(allRoles, contextID)
		body.Paths = m.authorizer.GetPaths(contextRoles, in.AppID)
		body.Capabilities = m.authorizer.GetCapabilities(contextRoles, in.AppID)
	}

	b, err := json.Marshal(body)
	if err != nil {
		return nil, core.TechnicalError().WithAmbit(coreauth.Ambit).
			WithCode(coreauth.CodeTokenEncryption).WithCause(err)
	}

	cipherText, err := core.Encrypt(b, in.AppID)
	if err != nil {
		return nil, core.TechnicalError().WithAmbit(coreauth.Ambit).
			WithCode(coreauth.CodeTokenEncryption).WithCause(err)
	}

	return &RawStringOutput{
		Body:        []byte(hex.EncodeToString(cipherText)),
		ContentType: "text/plain",
	}, nil
}
