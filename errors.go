package coreauth

// Ambit è l'ambito degli errori nati dentro questa libreria. Va messo con WithAmbit su ogni
// ApplicationError costruito qui: i costruttori base riempiono Ambit con l'AppName, cioè con
// l'applicazione che riceve l'errore, e senza sovrascriverlo un guasto della libreria si
// presenterebbe come un errore dell'applicazione.
const Ambit = "go-core-auth"

// Codici degli errori della libreria. La tabella completa è in ERRORI.md.
const (
	// CodeSourceLoad — la sorgente non ha potuto leggere l'ACL.
	CodeSourceLoad = "AUTH-SRC-LOAD"
	// CodeBootLoad — il primo caricamento è fallito con fail-fast-on-boot attivo.
	CodeBootLoad = "AUTH-BOOT-LOAD"
	// CodeForbiddenRole — nessuno dei ruoli presentati abilita la rotta.
	CodeForbiddenRole = "AUTH-FORBIDDEN"
	// CodeForbiddenCtx — il contesto presentato non è autorizzato per quei ruoli.
	CodeForbiddenCtx = "AUTH-CTX-FORBIDDEN"
	// CodeTokenEncryption — cifratura del token di sessione fallita.
	CodeTokenEncryption = "AUTH-TOKEN-CRYPT"
)
