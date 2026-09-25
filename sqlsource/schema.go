package sqlsource

import (
	"reflect"
	"sort"
	"strings"

	"github.com/uptrace/bun"
)

// Schema descrive le tabelle dell'ACL: nome della tabella → colonne, derivati per reflection dai
// modelli bun. È la stessa sorgente da cui nascono le query e la DDL, quindi non può divergerne.
//
// È esportata perché lo schema ha due lettori in due moduli diversi: sqlsource lo interroga e il
// generatore di seed di go-core-api lo scrive. Un nome di colonna cambiato da un lato si scopre
// solo se l'altro lato ha qualcosa da confrontare — altrimenti si scopre in produzione, come una
// colonna che non esiste.
func Schema() map[string][]string {
	out := make(map[string][]string, len(tables()))
	for _, model := range tables() {
		name, cols := describe(model)
		out[name] = cols
	}
	return out
}

// TableNames restituisce i nomi delle tabelle nell'ordine in cui vanno create.
func TableNames() []string {
	names := make([]string, 0, len(tables()))
	for _, model := range tables() {
		name, _ := describe(model)
		names = append(names, name)
	}
	return names
}

func describe(model any) (string, []string) {
	t := reflect.TypeOf(model).Elem()

	var table string
	var cols []string
	for i := range t.NumField() {
		f := t.Field(i)
		tag := f.Tag.Get("bun")

		if f.Type == reflect.TypeOf(bun.BaseModel{}) {
			table = strings.TrimPrefix(tag, "table:")
			continue
		}
		if name, _, _ := strings.Cut(tag, ","); name != "" {
			cols = append(cols, name)
		}
	}
	sort.Strings(cols)
	return table, cols
}
