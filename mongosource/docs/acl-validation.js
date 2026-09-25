// Validator della collection "acl".
//
// Verifica ciò che mongosource assume leggendo i documenti. Il costo di non averlo è che un refuso
// nel seed non si manifesta come errore di scrittura ma come un permesso che non arriva mai:
// un documento con _et sbagliato viene ignorato, uno senza category non appartiene ad alcuna
// categoria e quindi non risponde a nessuna domanda.
//
// validationAction 'error' rifiuta la scrittura; 'warn' la registra soltanto.

(function () {
  const validator = {
    $and: [
      { _et: { $in: ['CONTEXT', 'APP', 'ROLE', 'CAPABILITY', 'CAPABILITYGROUP'] } },

      // Una CAPABILITY dichiara sempre la sua categoria: senza, non è raggiungibile da nessuna
      // domanda dell'Authorizer.
      { $or: [
        { _et: { $ne: 'CAPABILITY' } },
        { category: { $in: ['api', 'ui', 'action_ui', 'action_api'] } },
      ]},

      // Il sottodocumento è quello della categoria: api su una capability "api", ui su una "ui".
      // Averli entrambi significa che uno dei due non verrà mai letto.
      { $or: [
        { _et: { $ne: 'CAPABILITY' } },
        { $and: [{ api: { $exists: true } }, { ui: { $exists: false } }, { category: 'api' }] },
        { $and: [{ ui: { $exists: true } }, { api: { $exists: false } }, { category: 'ui' }] },
        { $and: [{ ui: { $exists: false } }, { api: { $exists: false } }] },
      ]},

      // Un'APP dichiara il suo prefisso di rotta, altrimenti non è navigabile.
      { $or: [
        { _et: { $ne: 'APP' } },
        { path: { $type: 'string', $ne: '' } },
      ]},
    ],
  };

  db.runCommand({
    collMod: 'acl',
    validator: validator,
    validationLevel: 'moderate',
    validationAction: 'error',
  });

  print('[acl-validation] validator applicato');
})();
