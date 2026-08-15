import assert from "node:assert/strict";
import test from "node:test";
import {
    applyBps,
    BastilleClient,
    parseMoney,
    previewFunding,
    railHhiBps,
    ratioBps,
    serializeMoney,
} from "./BastilleClient.ts";

test("money parser accepts canonical integer inputs", () => {
    assert.equal(parseMoney(12n), 12n);
    assert.equal(parseMoney(12), 12n);
    assert.equal(parseMoney("12000000000000000000"), 12_000_000_000_000_000_000n);
    assert.throws(() => parseMoney(-1));
    assert.throws(() => parseMoney("01"));
    assert.throws(() => parseMoney(Number.MAX_SAFE_INTEGER + 1));
});

test("basis point helpers use integer rounding", () => {
    assert.equal(applyBps(101n, 500), 5n);
    assert.equal(applyBps(101n, 500, true), 6n);
    assert.equal(ratioBps(60n, 100n), 6_000);
    assert.throws(() => applyBps(10n, 10_001));
});

test("rail concentration uses exact integer squares", () => {
    assert.equal(railHhiBps([60n, 40n]), 5_200);
    assert.equal(railHhiBps([100n]), 10_000);
    assert.equal(railHhiBps([]), 0);
});

test("funding preview combines netting buffer and stress", () => {
    const preview = previewFunding(
        [
            {
                id: "i1",
                kind: "internal",
                sourceAccount: "operating",
                destinationAccount: "reserve",
                asset: "usdc",
                amount: 100n,
            },
            {
                id: "i2",
                kind: "internal",
                sourceAccount: "reserve",
                destinationAccount: "operating",
                asset: "usdc",
                amount: 40n,
            },
            {
                id: "i3",
                kind: "withdrawal",
                sourceAccount: "operating",
                rail: "swift",
                asset: "usdc",
                amount: 60n,
            },
        ],
        { reserveBufferBps: 1_000, railStressBps: { swift: 2_000 }, reserves: { usdc: 144n } },
    );
    assert.equal(preview.assets[0].internalGross, 140n);
    assert.equal(preview.assets[0].externalGross, 60n);
    assert.equal(preview.assets[0].peakAccountDebit, 120n);
    assert.equal(preview.assets[0].reserveRequired, 132n);
    assert.equal(preview.assets[0].stressedReserve, 144n);
    assert.equal(preview.ready, true);
});

test("funding preview keeps assets separate", () => {
    const preview = previewFunding(
        [
            {
                id: "u",
                kind: "withdrawal",
                sourceAccount: "operating",
                rail: "swift",
                asset: "usdc",
                amount: 25n,
            },
            {
                id: "e",
                kind: "withdrawal",
                sourceAccount: "operating",
                rail: "sepa",
                asset: "eurc",
                amount: 40n,
            },
        ],
        { reserves: { usdc: 100n, eurc: 100n } },
    );
    assert.equal(preview.assets.length, 2);
    assert.equal(preview.assets.find((asset) => asset.asset === "usdc")?.externalGross, 25n);
    assert.equal(preview.assets.find((asset) => asset.asset === "eurc")?.externalGross, 40n);
});

test("funding preview rejects ambiguous instructions", () => {
    assert.throws(() =>
        previewFunding([
            {
                id: "same",
                kind: "withdrawal",
                sourceAccount: "a",
                rail: "swift",
                asset: "usdc",
                amount: 10n,
            },
            {
                id: "same",
                kind: "withdrawal",
                sourceAccount: "b",
                rail: "sepa",
                asset: "usdc",
                amount: 10n,
            },
        ]),
    );
    assert.throws(() =>
        previewFunding([
            { id: "internal", kind: "internal", sourceAccount: "a", asset: "usdc", amount: 10n },
        ]),
    );
});

test("money serializer produces JSON-safe objects", () => {
    assert.deepEqual(serializeMoney({ gross: 10n, values: [2n, 3n] }), {
        gross: "10",
        values: ["2", "3"],
    });
});

test("client validates scenarios and derives health", async () => {
    const payload = JSON.stringify({
        name: "health",
        results: [],
        snapshot: {
            epoch: 12,
            balances: [{ account: "a", asset: "usdc", available: 100, reserved: 20 }],
            exposures: [{ capId: "cap", limit: 500, used: 100, remaining: 400 }],
            signers: [{ id: "s", status: "active", role: "risk" }],
            auditIssues: [],
        },
        report: { withdrawalTotal: 0, internalTotal: 0, executed: ["op"], rejected: [] },
    });
    const client = new BastilleClient(async (args) => {
        assert.deepEqual(args, ["run", "fixture.json"]);
        return payload;
    });
    assert.deepEqual(await client.health("fixture.json"), {
        epoch: 12,
        solvent: true,
        withinExposure: true,
        activeSigners: 1,
        executed: 1,
        rejected: 0,
        securitySignals: 0,
    });
});
