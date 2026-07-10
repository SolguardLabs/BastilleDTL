import { spawnSync } from "node:child_process";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
export const projectRoot = join(here, "..", "..");

export type ScenarioResult = {
  name: string;
  results: Array<Record<string, any>>;
  snapshot: {
    epoch: number;
    balances: Array<{ account: string; asset: string; available: number; reserved: number }>;
    approvals: Array<{
      id: string;
      operationId: string;
      signerId: string;
      partialHash: string;
      fullHash: string;
      consumedBy?: string[];
    }>;
    operations: Array<{
      operation: { id: string; kind: string; amount: number; sourceAccount: string };
      status: string;
      approvalIds?: string[];
      partialHash: string;
      fullHash: string;
    }>;
    exposures: Array<{ capId: string; limit: number; used: number; remaining: number }>;
    signers: Array<{ id: string; status: string; role: string; retiredEpoch?: number }>;
    auditIssues: Array<{ code: string; severity: string; operation?: string; approval?: string }>;
  };
  report: {
    withdrawalTotal: number;
    internalTotal: number;
    approvalReuseHits: number;
    executed: string[];
    rejected: string[];
  };
};

export function runFixture(name: string): ScenarioResult {
  const fixturePath = join(projectRoot, "tests", "fixtures", `${name}.json`);
  const child = spawnSync("go", ["run", "./cmd/bastilledtl", "run", fixturePath], {
    cwd: projectRoot,
    encoding: "utf8",
  });
  if (child.status !== 0) {
    throw new Error(
      [
        `fixture ${name} failed`,
        `status: ${child.status}`,
        `stdout: ${child.stdout}`,
        `stderr: ${child.stderr}`,
      ].join("\n"),
    );
  }
  return JSON.parse(child.stdout) as ScenarioResult;
}

export function resultByLabel(result: ScenarioResult, label: string): Record<string, any> {
  const found = result.results.find((entry) => entry.label === label);
  if (!found) {
    throw new Error(`missing action label ${label}`);
  }
  return found;
}

export function balanceOf(
  result: ScenarioResult,
  account: string,
  asset: string,
): { available: number; reserved: number } {
  const found = result.snapshot.balances.find(
    (balance) => balance.account === account && balance.asset === asset,
  );
  return found ?? { account, asset, available: 0, reserved: 0 };
}

export function exposureOf(
  result: ScenarioResult,
  capId: string,
): { capId: string; limit: number; used: number; remaining: number } {
  const found = result.snapshot.exposures.find((exposure) => exposure.capId === capId);
  if (!found) {
    throw new Error(`missing exposure cap ${capId}`);
  }
  return found;
}
