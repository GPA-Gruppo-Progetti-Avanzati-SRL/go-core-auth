// Indici della collection "acl".
//
// Il percorso di lettura di go-core-auth NON ha bisogno di indici: mongosource legge l'ACL intero
// con una sola find({}) e lo tiene in memoria fino al refresh successivo. Un indice qui non serve
// a quella query e non va aggiunto per abitudine.
//
// Gli indici sotto servono all'AMMINISTRAZIONE dell'ACL — i tool che lo ispezionano o lo
// modificano per tipo di entità — e sono quindi facoltativi.

(function () {
  const coll = db.getCollection('acl');

  // Filtro per tipo di entità: la query che fa ogni strumento di amministrazione.
  coll.createIndex({ _et: 1 }, { name: 'ix_acl_et' });

  // Ruoli di un contesto.
  coll.createIndex(
    { _et: 1, _cid: 1 },
    { name: 'ix_acl_role_ctx', partialFilterExpression: { _et: 'ROLE' } }
  );

  // "Chi usa questa capability?": membership nei gruppi e nei ruoli.
  coll.createIndex({ capabilities: 1 }, { name: 'ix_acl_capabilities' });
  coll.createIndex({ capability_groups: 1 }, { name: 'ix_acl_capability_groups' });

  // Capability di un'app, per categoria.
  coll.createIndex(
    { _et: 1, category: 1, appId: 1 },
    { name: 'ix_acl_cap_app', partialFilterExpression: { _et: 'CAPABILITY' } }
  );

  print('[acl-indexes] indici creati');
})();
