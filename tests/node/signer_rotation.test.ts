import assert from "node:assert/strict";
import test from "node:test";
import { resultByLabel, runFixture } from "../helpers/runner.ts";

test("rotates signers and rejects approvals from retired signer", () => {
    const result = runFixture("signer_rotation");
    const rotation = resultByLabel(result, "rotate-alice").rotation as {
        retired: { id: string; status: string };
        activated: { id: string; status: string };
    };
    const rejected = resultByLabel(result, "old-alice-approval").error as { code: string };
    const diana = resultByLabel(result, "diana-approval").approval as { signerId: string };

    assert.equal(rotation.retired.id, "signer:alice");
    assert.equal(rotation.retired.status, "retired");
    assert.equal(rotation.activated.id, "signer:diana");
    assert.equal(rejected.code, "signer_inactive");
    assert.equal(diana.signerId, "signer:diana");
});
