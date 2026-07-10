import assert from "node:assert/strict";
import test from "node:test";
import { balanceOf, resultByLabel, runFixture } from "../helpers/runner.ts";

test("demonstrates approval reuse from internal movement to external withdrawal", () => {
  const result = runFixture("withdrawal_reuse");
  const internal = resultByLabel(result, "execute-internal").execution as {
    partialHash: string;
    fullHash: string;
    approvalBundle: { reuseDetected: boolean };
  };
  const withdrawal = resultByLabel(result, "execute-external").execution as {
    partialHash: string;
    fullHash: string;
    externalAddress: string;
    approvalBundle: { reuseDetected: boolean; accepted: Array<{ operationId: string }> };
  };
  const operating = balanceOf(result, "acct:alpha:operating", "usdc");

  assert.equal(withdrawal.partialHash, internal.partialHash);
  assert.notEqual(withdrawal.fullHash, internal.fullHash);
  assert.equal(withdrawal.approvalBundle.reuseDetected, true);
  assert.equal(withdrawal.approvalBundle.accepted[0].operationId, "op:reuse:internal");
  assert.equal(withdrawal.externalAddress, "beneficiary:outside-bank");
  assert.equal(operating.available, 760000);
  assert.equal(result.report.approvalReuseHits, 2);
  assert.ok(result.snapshot.auditIssues.some((issue) => issue.code === "approval_operation_mismatch"));
});
