import assert from "node:assert/strict";
import test from "node:test";
import { balanceOf, resultByLabel, runFixture } from "../helpers/runner.ts";

test("executes independently approved internal and external instructions", () => {
    const result = runFixture("settlement_sequence");
    const internal = resultByLabel(result, "execute-internal").execution as {
        approvalBundle: { operationBound: boolean; accepted: Array<{ operationId: string }> };
    };
    const withdrawal = resultByLabel(result, "execute-withdrawal").execution as {
        externalAddress: string;
        approvalBundle: { operationBound: boolean; accepted: Array<{ operationId: string }> };
    };

    assert.equal(internal.approvalBundle.operationBound, true);
    assert.equal(withdrawal.approvalBundle.operationBound, true);
    assert.ok(
        internal.approvalBundle.accepted.every(
            (approval) => approval.operationId === "op:sequence:internal",
        ),
    );
    assert.ok(
        withdrawal.approvalBundle.accepted.every(
            (approval) => approval.operationId === "op:sequence:withdrawal",
        ),
    );
    assert.equal(withdrawal.externalAddress, "beneficiary:clearing-bank");
    assert.equal(balanceOf(result, "acct:alpha:operating", "usdc").available, 800000);
    assert.equal(result.report.internalTotal, 120000);
    assert.equal(result.report.withdrawalTotal, 80000);
    assert.equal(result.report.approvalReuseHits, 0);
    assert.equal(result.snapshot.auditIssues.length, 0);
});
