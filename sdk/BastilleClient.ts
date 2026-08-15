export type MoneyInput = bigint | number | string;

export type FundingInstruction = {
    id: string;
    kind: "internal" | "withdrawal";
    sourceAccount: string;
    destinationAccount?: string;
    rail?: string;
    asset: string;
    amount: MoneyInput;
};

export type FundingOptions = {
    reserveBufferBps?: number;
    defaultRailStressBps?: number;
    railStressBps?: Record<string, number>;
    reserves?: Record<string, MoneyInput>;
};

export type AccountFundingPosition = {
    account: string;
    asset: string;
    grossDebit: bigint;
    grossCredit: bigint;
    netDebit: bigint;
    netCredit: bigint;
};

export type RailFundingSummary = {
    asset: string;
    rail: string;
    gross: bigint;
    shareBps: number;
    stressBps: number;
    stressAddOn: bigint;
};

export type AssetFundingSummary = {
    asset: string;
    internalGross: bigint;
    externalGross: bigint;
    peakAccountDebit: bigint;
    reserveRequired: bigint;
    stressedReserve: bigint;
    availableReserve: bigint;
    shortfall: bigint;
    railHhiBps: number;
};

export type FundingPreview = {
    ready: boolean;
    instructionCount: number;
    positions: AccountFundingPosition[];
    rails: RailFundingSummary[];
    assets: AssetFundingSummary[];
};

export type ScenarioResult = {
    name: string;
    results: Array<Record<string, unknown>>;
    snapshot: {
        epoch: number;
        balances: Array<{ account: string; asset: string; available: number; reserved: number }>;
        exposures: Array<{ capId: string; limit: number; used: number; remaining: number }>;
        signers: Array<{ id: string; status: string; role: string }>;
        auditIssues: Array<{ code: string; severity: string }>;
    };
    report: {
        withdrawalTotal: number;
        internalTotal: number;
        executed: string[];
        rejected: string[];
    };
};

export const BPS = 10_000n;

export function parseMoney(value: MoneyInput, field = "amount"): bigint {
    if (typeof value === "bigint") {
        if (value < 0n) throw new RangeError(`${field} must be non-negative`);
        return value;
    }
    if (typeof value === "number") {
        if (!Number.isSafeInteger(value) || value < 0) {
            throw new RangeError(`${field} must be a non-negative safe integer`);
        }
        return BigInt(value);
    }
    if (!/^(0|[1-9][0-9]*)$/.test(value)) {
        throw new TypeError(`${field} must be an unsigned integer string`);
    }
    return BigInt(value);
}

export function applyBps(value: MoneyInput, bps: number, roundUp = false): bigint {
    assertBps(bps, "bps");
    const numerator = parseMoney(value) * BigInt(bps);
    return roundUp && numerator > 0n ? (numerator + BPS - 1n) / BPS : numerator / BPS;
}

export function ratioBps(part: MoneyInput, total: MoneyInput): number {
    const numerator = parseMoney(part, "part");
    const denominator = parseMoney(total, "total");
    if (denominator === 0n) return 0;
    const ratio = (numerator * BPS) / denominator;
    return Number(ratio > BPS ? BPS : ratio);
}

export function railHhiBps(values: Iterable<MoneyInput>): number {
    let total = 0n;
    let squares = 0n;
    for (const input of values) {
        const value = parseMoney(input);
        total += value;
        squares += value * value;
    }
    if (total === 0n) return 0;
    return Number((squares * BPS) / (total * total));
}

export function previewFunding(
    instructions: FundingInstruction[],
    options: FundingOptions = {},
): FundingPreview {
    const reserveBufferBps = options.reserveBufferBps ?? 0;
    const defaultRailStressBps = options.defaultRailStressBps ?? 500;
    assertBps(reserveBufferBps, "reserveBufferBps");
    assertBps(defaultRailStressBps, "defaultRailStressBps");

    const references = new Set<string>();
    const debits = new Map<string, AccountFundingPosition>();
    const internalGross = new Map<string, bigint>();
    const externalGross = new Map<string, bigint>();
    const railGross = new Map<string, bigint>();

    for (const instruction of instructions) {
        validateInstruction(instruction);
        if (references.has(instruction.id)) throw new RangeError("instruction id is duplicated");
        references.add(instruction.id);
        const amount = parseMoney(instruction.amount);
        addPosition(debits, instruction.sourceAccount, instruction.asset, amount, 0n);
        if (instruction.kind === "internal") {
            addPosition(debits, instruction.destinationAccount!, instruction.asset, 0n, amount);
            addTotal(internalGross, instruction.asset, amount);
        } else {
            addTotal(externalGross, instruction.asset, amount);
            addTotal(
                railGross,
                `${instruction.asset}\u0000${instruction.rail!.toLowerCase()}`,
                amount,
            );
        }
    }

    const positions = [...debits.values()]
        .map((position) => ({
            ...position,
            netDebit:
                position.grossDebit > position.grossCredit
                    ? position.grossDebit - position.grossCredit
                    : 0n,
            netCredit:
                position.grossCredit > position.grossDebit
                    ? position.grossCredit - position.grossDebit
                    : 0n,
        }))
        .sort((left, right) =>
            `${left.asset}:${left.account}`.localeCompare(`${right.asset}:${right.account}`),
        );

    const rails = [...railGross.entries()]
        .map(([key, gross]) => {
            const [asset, rail] = key.split("\u0000");
            const stressBps = options.railStressBps?.[rail] ?? defaultRailStressBps;
            assertBps(stressBps, `railStressBps.${rail}`);
            return {
                asset,
                rail,
                gross,
                shareBps: ratioBps(gross, externalGross.get(asset) ?? 0n),
                stressBps,
                stressAddOn: applyBps(gross, stressBps, true),
            };
        })
        .sort((left, right) =>
            `${left.asset}:${left.rail}`.localeCompare(`${right.asset}:${right.rail}`),
        );

    const assets = [...new Set([...internalGross.keys(), ...externalGross.keys()])]
        .map((asset) => {
            const peak = positions
                .filter((position) => position.asset === asset)
                .reduce(
                    (maximum, position) =>
                        position.netDebit > maximum ? position.netDebit : maximum,
                    0n,
                );
            const external = externalGross.get(asset) ?? 0n;
            const base = peak > external ? peak : external;
            const reserveRequired = base + applyBps(base, reserveBufferBps, true);
            const assetRails = rails.filter((rail) => rail.asset === asset);
            const stress = assetRails.reduce((sum, rail) => sum + rail.stressAddOn, 0n);
            const stressedReserve = reserveRequired + stress;
            const availableReserve = parseMoney(
                options.reserves?.[asset] ?? 0n,
                `reserves.${asset}`,
            );
            return {
                asset,
                internalGross: internalGross.get(asset) ?? 0n,
                externalGross: external,
                peakAccountDebit: peak,
                reserveRequired,
                stressedReserve,
                availableReserve,
                shortfall:
                    stressedReserve > availableReserve ? stressedReserve - availableReserve : 0n,
                railHhiBps: railHhiBps(assetRails.map((rail) => rail.gross)),
            };
        })
        .sort((left, right) => left.asset.localeCompare(right.asset));

    return {
        ready: assets.every((asset) => asset.shortfall === 0n),
        instructionCount: instructions.length,
        positions,
        rails,
        assets,
    };
}

export function serializeMoney(value: unknown): unknown {
    if (typeof value === "bigint") return value.toString();
    if (Array.isArray(value)) return value.map(serializeMoney);
    if (value && typeof value === "object") {
        return Object.fromEntries(
            Object.entries(value).map(([key, item]) => [key, serializeMoney(item)]),
        );
    }
    return value;
}

export class BastilleClient {
    constructor(private readonly executor: (args: readonly string[]) => Promise<string>) {
        if (typeof executor !== "function") throw new TypeError("executor must be a function");
    }

    async runFixture(path: string): Promise<ScenarioResult> {
        if (!path || path.includes("\0")) throw new TypeError("fixture path is invalid");
        const output = await this.executor(["run", path]);
        const result = JSON.parse(output) as ScenarioResult;
        validateScenario(result);
        return result;
    }

    async health(path: string): Promise<{
        epoch: number;
        solvent: boolean;
        withinExposure: boolean;
        activeSigners: number;
        executed: number;
        rejected: number;
        securitySignals: number;
    }> {
        const result = await this.runFixture(path);
        return {
            epoch: result.snapshot.epoch,
            solvent: result.snapshot.balances.every(
                (balance) => balance.available >= 0 && balance.reserved >= 0,
            ),
            withinExposure: result.snapshot.exposures.every(
                (exposure) => exposure.used <= exposure.limit,
            ),
            activeSigners: result.snapshot.signers.filter((signer) => signer.status === "active")
                .length,
            executed: result.report.executed.length,
            rejected: result.report.rejected.length,
            securitySignals: result.snapshot.auditIssues.length,
        };
    }
}

function assertBps(value: number, field: string): void {
    if (!Number.isInteger(value) || value < 0 || value > 10_000) {
        throw new RangeError(`${field} must be an integer between 0 and 10000`);
    }
}

function validateInstruction(instruction: FundingInstruction): void {
    if (!instruction.id || !instruction.sourceAccount || !instruction.asset) {
        throw new TypeError("instruction identity fields are required");
    }
    if (parseMoney(instruction.amount) === 0n)
        throw new RangeError("instruction amount must be positive");
    if (instruction.kind === "internal") {
        if (
            !instruction.destinationAccount ||
            instruction.destinationAccount === instruction.sourceAccount
        ) {
            throw new TypeError("internal instruction requires a distinct destination account");
        }
    } else if (instruction.kind === "withdrawal") {
        if (!instruction.rail) throw new TypeError("withdrawal instruction requires a rail");
    } else {
        throw new TypeError("instruction kind is unsupported");
    }
}

function addPosition(
    positions: Map<string, AccountFundingPosition>,
    account: string,
    asset: string,
    debit: bigint,
    credit: bigint,
): void {
    const key = `${asset}\u0000${account}`;
    const current = positions.get(key) ?? {
        account,
        asset,
        grossDebit: 0n,
        grossCredit: 0n,
        netDebit: 0n,
        netCredit: 0n,
    };
    current.grossDebit += debit;
    current.grossCredit += credit;
    positions.set(key, current);
}

function addTotal(values: Map<string, bigint>, key: string, value: bigint): void {
    values.set(key, (values.get(key) ?? 0n) + value);
}

function validateScenario(result: ScenarioResult): void {
    if (!result || typeof result.name !== "string" || !Array.isArray(result.results)) {
        throw new TypeError("scenario result shape is invalid");
    }
    if (
        !result.snapshot ||
        !Array.isArray(result.snapshot.balances) ||
        !Array.isArray(result.snapshot.exposures)
    ) {
        throw new TypeError("scenario snapshot shape is invalid");
    }
    if (
        !result.report ||
        !Array.isArray(result.report.executed) ||
        !Array.isArray(result.report.rejected)
    ) {
        throw new TypeError("scenario report shape is invalid");
    }
}
