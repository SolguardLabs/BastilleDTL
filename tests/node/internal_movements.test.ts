import assert from "node:assert/strict";
import test from "node:test";
import { balanceOf, exposureOf, resultByLabel, runFixture } from "../helpers/runner.ts";

test("executes approved internal movements and updates ledger/exposure", () => {
    const result = runFixture("internal_movements");
    const executed = resultByLabel(result, "execute-internal").execution as {
        operation: string;
        approvalBundle: { accepted: Array<{ id: string }>; reuseDetected: boolean };
    };
    const operating = balanceOf(result, "acct:alpha:operating", "usdc");
    const reserve = balanceOf(result, "acct:alpha:reserve", "usdc");
    const cap = exposureOf(result, "cap:alpha:usdc:internal");

    assert.equal(executed.operation, "op:internal:rebalance");
    assert.equal(executed.approvalBundle.accepted.length, 2);
    assert.equal(executed.approvalBundle.reuseDetected, false);
    assert.equal(operating.available, 880000);
    assert.equal(reserve.available, 220000);
    assert.equal(cap.used, 120000);
});
