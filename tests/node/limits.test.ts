import assert from "node:assert/strict";
import test from "node:test";
import { exposureOf, resultByLabel, runFixture } from "../helpers/runner.ts";

test("rejects withdrawals beyond exposure caps", () => {
    const result = runFixture("limits");
    const rejected = resultByLabel(result, "oversized-withdrawal");
    const error = rejected.error as { code: string; message: string };
    const cap = exposureOf(result, "cap:alpha:usdc:withdrawal");

    assert.equal(error.code, "limit_exceeded");
    assert.match(error.message, /daily cap|withdrawal amount/);
    assert.equal(cap.used, 0);
    assert.equal(result.report.withdrawalTotal, 0);
});
