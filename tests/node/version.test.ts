import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import test from "node:test";
import { projectRoot } from "../helpers/runner.ts";

test("reports protocol and schema versions", () => {
    const child = spawnSync("go", ["run", "./cmd/bastilledtl", "version"], {
        cwd: projectRoot,
        encoding: "utf8",
    });
    assert.equal(child.status, 0, child.stderr);
    assert.deepEqual(JSON.parse(child.stdout), {
        protocol: "BastilleDTL",
        version: "1.0.0",
        schemaVersion: "bastille/v1",
    });
});
