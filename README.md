# BastilleDTL

![banner](./assets/banner.png)

BastilleDTL es una infraestructura Go de limites institucionales para mesas de
tesoreria. Modela cuentas, roles, exposure caps, rotacion de firmantes,
movimientos internos y retiros externos con aprobacion compuesta.

El lab esta construido como CTF de logica economica. Los tests TypeScript
ejecutan escenarios JSON contra el binario Go y validan que los controles
normales funcionan, mientras queda una vulnerabilidad critica en la
reutilizacion de aprobaciones.

## Componentes

- `src/domain`: contratos de negocio, roles, errores, operaciones y snapshots.
- `src/ledger`: balances disponibles, reservas y diario contable.
- `src/exposure`: caps por institucion, cuenta, activo y rail externo.
- `src/authz`: firmantes, rotacion, aprobaciones y verificacion compuesta.
- `src/engine`: servicio de aplicacion, ejecucion y auditoria.
- `src/scenario`: loader y runner determinista para fixtures de tests.
- `tests/node`: tests TypeScript de integracion.

## Uso

```bash
npm install
npm test
go run ./cmd/bastilledtl run tests/fixtures/withdrawal_reuse.json
```

## Lab

Una operacion grande requiere dos aprobaciones de firmantes activos. El sistema
calcula un hash economico parcial para agrupar aprobaciones, pero omite campos
como tipo de operacion, destino interno, beneficiario externo y rail. Una
aprobacion legitima para un movimiento interno puede reutilizarse para un retiro
externo con el mismo importe, activo y cuenta origen.
