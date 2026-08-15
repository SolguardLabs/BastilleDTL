# Integración

## Superficies

BastilleDTL ofrece:

1. paquetes Go para integración nativa;
2. CLI JSON para procesos externos;
3. SDK TypeScript para parsing y previsión de tesorería.

El SDK no gestiona claves ni reemplaza las validaciones del engine.

## Contrato CLI

```text
bastilledtl run <scenario.json>
bastilledtl default
bastilledtl version
```

- stdout contiene un documento JSON;
- stderr contiene diagnóstico;
- código `0` confirma finalización;
- otro código indica rechazo;
- una salida parcial no se acepta.

El adaptador impone timeout, límite de bytes y esquema.

## Resultado de escenario

| Campo                  | Tipo    | Uso                        |
| ---------------------- | ------- | -------------------------- |
| `name`                 | string  | nombre de ejecución        |
| `results`              | array   | resultado por acción       |
| `snapshot.epoch`       | integer | epoch final                |
| `snapshot.balances`    | array   | available/reserved         |
| `snapshot.exposures`   | array   | limit/used/remaining       |
| `snapshot.signers`     | array   | identidades y status       |
| `snapshot.approvals`   | array   | decisiones, hashes y usos  |
| `snapshot.operations`  | array   | records terminales         |
| `snapshot.events`      | array   | trazabilidad               |
| `snapshot.auditIssues` | array   | señales de control         |
| `report`               | object  | totales y listas agregadas |

Los consumidores toleran campos nuevos y rechazan ausencia o cambio de tipo de
campos requeridos.

## Adaptador Node

```ts
import { spawn } from "node:child_process";
import { BastilleClient } from "./sdk/BastilleClient.ts";

function execute(args: readonly string[]): Promise<string> {
    return new Promise((resolve, reject) => {
        const child = spawn("bastilledtl", [...args], {
            stdio: ["ignore", "pipe", "pipe"],
            windowsHide: true,
        });
        let stdout = "";
        let stderr = "";
        child.stdout.setEncoding("utf8");
        child.stderr.setEncoding("utf8");
        child.stdout.on("data", (chunk) => (stdout += chunk));
        child.stderr.on("data", (chunk) => (stderr += chunk));
        child.once("error", reject);
        child.once("close", (code) => {
            if (code === 0) resolve(stdout);
            else reject(new Error(`bastille exited with code ${code}: ${stderr}`));
        });
    });
}

const client = new BastilleClient(execute);
const health = await client.health("scenario.json");
if (!health.solvent || !health.withinExposure) {
    throw new Error("operational invariants rejected");
}
```

En un servicio persistente, añada cancelación, límite de memoria y longitud
máxima de stdout/stderr.

## Importes

`MoneyInput` acepta:

- `bigint` no negativo;
- `number` entero seguro no negativo;
- string decimal canónica.

Rechaza:

```text
-1
1.5
"+10"
"01"
"1e6"
Number.MAX_SAFE_INTEGER + 1
```

Cruzar JSON como string:

```ts
import { serializeMoney } from "./sdk/BastilleClient.ts";

const payload = JSON.stringify(serializeMoney(preview));
```

## Preview de liquidez

```ts
import { previewFunding } from "./sdk/BastilleClient.ts";

const plan = previewFunding(
    [
        {
            id: "w1",
            kind: "withdrawal",
            sourceAccount: "operating",
            rail: "swift",
            asset: "usdc",
            amount: "60000",
        },
        {
            id: "w2",
            kind: "withdrawal",
            sourceAccount: "settlement",
            rail: "sepa",
            asset: "usdc",
            amount: "40000",
        },
    ],
    {
        reserveBufferBps: 500,
        railStressBps: { swift: 1_000, sepa: 500 },
        reserves: { usdc: "120000" },
    },
);

console.log(plan.assets[0].railHhiBps); // 5200
console.log(plan.ready); // true
```

La proyección es una ayuda de preflight. Go debe recalcular con el estado
vigente antes de ejecutar.

## Integración Go

### Servicio

```go
bootstrap := scenario.DefaultBootstrap()
service, err := engine.NewService(bootstrap)
if err != nil {
    return err
}

approval, err := service.Approve(request)
receipt, err := service.Execute(operation.CloneWithApprovals([]domain.ApprovalID{approval.ID}))
```

La integración debe manejar `domain.Error` por código y evitar comparar strings
de mensaje.

### Planificador

```go
planner, err := treasury.NewPlanner(treasury.Limits{
    ReserveBufferBps: 1000,
    MaxRailShareBps:  7000,
})
plan, err := planner.Preview(window, instructions, reserves)
if err != nil {
    return err
}
if !plan.Ready {
    return fmt.Errorf("funding plan requires intervention: %v", plan.Reasons)
}
```

### Comité

```go
committee, err := control.NewCommittee(quorum, reviewers)
digest, _, err := committee.Submit(signedChange)
outcome, err := committee.Vote(signedVote)
change, err := committee.Execute(digest, epoch)
```

El payload real se busca por `change.ConfigDigest` y se vuelve a verificar.

## Idempotencia

Ante timeout:

1. no crear un nuevo operation ID de inmediato;
2. consultar snapshot y journal;
3. buscar el ID original;
4. comparar receipt y ledger entry;
5. reintentar solo si no existe estado terminal;
6. evitar ciclos automáticos sin límite.

Las etiquetas de fixture son locales al runner y no sustituyen operation IDs.

## Errores

Los códigos incluyen:

```text
invalid_request
not_found
permission_denied
limit_exceeded
insufficient_funds
approval_required
approval_rejected
signer_inactive
signer_conflict
operation_rejected
account_blocked
exposure_exceeded
invariant_violation
rotation_window_closed
```

El cliente clasifica por código, registra ID y aplica una política de reintento
acotada.

## Compatibilidad

- Los campos actuales mantienen tipo dentro de `1.x`.
- Se pueden añadir campos opcionales en minor releases.
- Cambios de semántica firmada requieren dominio nuevo.
- Cambios incompatibles de JSON requieren versión mayor.
- `package.json`, `src/version/info.go` y tag deben coincidir.

## Checklist

- [ ] Importes como strings o bigint.
- [ ] Timeout, cancelación y límite de salida.
- [ ] stdout separado de stderr.
- [ ] Código de salida comprobado.
- [ ] IDs persistidos para idempotencia.
- [ ] Señales de auditoría monitorizadas.
- [ ] Claves fuera de logs y fixtures.
- [ ] Versión y commit en telemetría.
- [ ] Reintentos acotados.
