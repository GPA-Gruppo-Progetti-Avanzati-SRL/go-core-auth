// Pre-popolamento della collection "acl" letta da go-core-auth/mongosource.
//
// Uso (mongosh):
//   use <database>
//   load('docs/acl-seed.js')
//
// Lo script è idempotente: ogni documento è scritto con replaceOne + upsert su _id.
// Metti RESET a true per ripartire da una collection vuota.

(function () {
  const coll = db.getCollection('acl');
  const RESET = false;

  if (RESET) {
    print('[acl-seed] deleteMany({})');
    coll.deleteMany({});
  }

  function upsert(doc) {
    coll.replaceOne({ _id: doc._id }, doc, { upsert: true });
  }

  // --- CONTEXT ---------------------------------------------------------------
  // home_app designa l'app home del contesto. È il solo modo di dichiararla: senza,
  // il contesto ricade sull'app con path "/".
  const contexts = [
    { _id: 'NORD', _et: 'CONTEXT', label: 'Area Nord', home_app: 'APP_HOME_NORD', order: 1 },
    { _id: 'SUD',  _et: 'CONTEXT', label: 'Area Sud',  home_app: 'APP_HOME_SUD',  order: 2 },
  ];

  // --- APP -------------------------------------------------------------------
  // path è il prefisso di rotta dell'app: "/" per una home, "/nome/" per le altre.
  // Con un contesto selezionato l'Authorizer antepone "/{contesto}".
  const apps = [
    { _id: 'APP_HOME_NORD', _et: 'APP', description: 'Home Nord', path: '/', order: 1 },
    { _id: 'APP_HOME_SUD',  _et: 'APP', description: 'Home Sud',  path: '/', order: 2 },
    { _id: 'APP_ANAG',      _et: 'APP', description: 'Anagrafica', path: '/anagrafica/', icon: 'people', order: 3 },
  ];

  // --- CAPABILITY ------------------------------------------------------------
  // category: api | ui | action_ui | action_api.
  //   api        → autorizza una richiesta HTTP: sottodocumento api { path, methods }
  //                (methods assente = tutti i metodi). operationid resta per gli ACL
  //                scritti sull'operationId di huma.
  //   ui         → voce di menu: sottodocumento ui { endpoint, icon, order, menu }
  //   action_ui  → comando dell'interfaccia
  //   action_api → azione verificata dalla business logic con HasCapability
  // appId assente = la capability vale per ogni app.
  const capabilities = [
    { _id: 'API_PERSONS_READ', _et: 'CAPABILITY', category: 'api', description: 'Lettura anagrafica',
      api: { operationid: 'getPersons', path: '/api/persons/**', methods: ['GET'] } },
    { _id: 'API_PERSONS_WRITE', _et: 'CAPABILITY', category: 'api', description: 'Scrittura anagrafica',
      api: { operationid: 'createPerson', path: '/api/persons/**', methods: ['POST', 'PUT', 'DELETE'] } },

    { _id: 'UI_ANAG_LIST', _et: 'CAPABILITY', category: 'ui', description: 'Anagrafica', appId: 'APP_ANAG',
      ui: { endpoint: '/persons', icon: 'people', order: 1, menu: true } },
    { _id: 'UI_ABOUT', _et: 'CAPABILITY', category: 'ui', description: 'Informazioni',
      ui: { endpoint: '/about', icon: 'info', order: 99, menu: true } },

    { _id: 'ACT_PERSON_EDIT', _et: 'CAPABILITY', category: 'action_ui', description: 'Modifica anagrafica', appId: 'APP_ANAG' },

    { _id: 'ACT_PERSON_EXPORT', _et: 'CAPABILITY', category: 'action_api', description: 'Export massivo' },
  ];

  // --- CAPABILITYGROUP -------------------------------------------------------
  const groups = [
    { _id: 'GRP_ANAG_READ',  _et: 'CAPABILITYGROUP', description: 'Anagrafica sola lettura',
      capabilities: ['API_PERSONS_READ', 'UI_ANAG_LIST'] },
    { _id: 'GRP_ANAG_WRITE', _et: 'CAPABILITYGROUP', description: 'Anagrafica completa',
      capabilities: ['API_PERSONS_READ', 'API_PERSONS_WRITE', 'UI_ANAG_LIST', 'ACT_PERSON_EDIT'] },
  ];

  // --- ROLE ------------------------------------------------------------------
  // _cid lega il ruolo a un contesto. Un ruolo senza _cid è context-agnostic: vale su
  // ogni contesto e li espone tutti.
  const roles = [
    { _id: 'OPERATORE_NORD', _et: 'ROLE', _cid: 'NORD', description: 'Operatore Nord',
      capability_groups: ['GRP_ANAG_READ'] },
    { _id: 'OPERATORE_SUD',  _et: 'ROLE', _cid: 'SUD',  description: 'Operatore Sud',
      capability_groups: ['GRP_ANAG_WRITE'] },
    { _id: 'AMMINISTRATORE', _et: 'ROLE', description: 'Amministratore (ogni contesto)',
      capability_groups: ['GRP_ANAG_WRITE'], capabilities: ['UI_ABOUT', 'ACT_PERSON_EXPORT'] },
  ];

  [contexts, apps, capabilities, groups, roles].forEach(function (list) {
    list.forEach(upsert);
  });

  print('[acl-seed] contesti=' + contexts.length + ' app=' + apps.length +
        ' capability=' + capabilities.length + ' gruppi=' + groups.length + ' ruoli=' + roles.length);
})();
