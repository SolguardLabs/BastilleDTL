import assert from "node:assert/strict";
import test from "node:test";
import { resultByLabel, runFixture } from "../helpers/runner.ts";

test("enforces signer roles for approvals", () => {
    const result = runFixture("roles");
    const rejected = resultByLabel(result, "auditor-approval");
    const error = rejected.error as { code: string; message: string };
    const alice = resultByLabel(result, "alice-approval").approval as { signerId: string };

    assert.equal(error.code, "permission_denied");
    assert.equal(alice.signerId, "signer:alice");
    assert.equal(result.snapshot.approvals.length, 1);
});
