package coreauth

import "strings"

// matchGlob confronta un pattern di rotta dichiarato nell'ACL con il path di una richiesta reale.
//
// Forme ammesse:
//   - /api/**                  qualsiasi path sotto /api/ (e /api stesso)
//   - /api/persons/*           un solo segmento jolly
//   - /api/persons/:id         path param nominale (un segmento)
//   - /api/persons/{id}        path param in stile OpenAPI (un segmento)
//   - /api/persons/:id/orders  param in mezzo al path
//   - /api/persons             corrispondenza esatta
func matchGlob(pattern, path string) bool {
	if pattern == path {
		return true
	}
	// "/**" in coda: prefisso libero.
	if prefix, ok := strings.CutSuffix(pattern, "/**"); ok {
		return path == prefix || strings.HasPrefix(path, prefix+"/")
	}
	pp := strings.Split(strings.Trim(pattern, "/"), "/")
	rp := strings.Split(strings.Trim(path, "/"), "/")
	if len(pp) != len(rp) {
		return false
	}
	for i := range pp {
		seg := pp[i]
		if seg == "*" || strings.HasPrefix(seg, ":") ||
			(strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}")) {
			continue // jolly o path param: qualsiasi valore
		}
		if seg != rp[i] {
			return false
		}
	}
	return true
}

// matchMethods verifica il metodo HTTP contro quelli dichiarati dalla capability.
// Una lista vuota significa "tutti i metodi": è il default di una capability che non li nomina.
func matchMethods(methods []string, method string) bool {
	if len(methods) == 0 {
		return true
	}
	for _, m := range methods {
		if strings.EqualFold(m, method) {
			return true
		}
	}
	return false
}
