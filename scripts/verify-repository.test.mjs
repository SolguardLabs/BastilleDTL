import { strict as assert } from "node:assert";
import { readFile, readdir } from "node:fs/promises";
import { join } from "node:path";
import test from "node:test";

const root = new URL("../", import.meta.url);
const expectedDocs = [
    "arquitectura.md",
    "despliegue.md",
    "gobierno.md",
    "integracion.md",
    "modelo-economico.md",
    "modelo-seguridad.md",
    "operaciones.md",
];

test("documentation surface is complete", async () => {
    const docs = (await readdir(new URL("docs/", root))).sort();
    assert.deepEqual(docs, expectedDocs);
    const readme = await readFile(new URL("README.md", root), "utf8");
    for (const document of expectedDocs) {
        assert.match(readme, new RegExp(`docs/${document.replace(".", "\\.")}`));
    }
    assert.match(readme, /```mermaid/);
});

test("release versions and workflows agree", async () => {
    const packageManifest = JSON.parse(await readFile(new URL("package.json", root), "utf8"));
    const versionSource = await readFile(new URL("src/version/info.go", root), "utf8");
    const integrity = await readFile(
        new URL(".github/workflows/release-integrity.yml", root),
        "utf8",
    );
    const ci = await readFile(new URL(".github/workflows/ci.yml", root), "utf8");

    assert.equal(packageManifest.version, "1.0.0");
    assert.match(versionSource, /Version\s+= "1\.0\.0"/);
    assert.match(integrity, /origin\/production/);
    assert.match(integrity, /cat-file -t/);
    assert.match(ci, /"v\*"/);
});

test("banner and public narrative meet repository policy", async () => {
    const banner = await readFile(new URL("assets/banner.png", root));
    assert.equal(banner.readUInt32BE(16), 1672);
    assert.equal(banner.readUInt32BE(20), 941);

    const blocked = [
        ["c", "t", "f"].join(""),
        ["l", "a", "b"].join(""),
        ["b", "u", "g"].join(""),
        ["e", "x", "p", "l", "o", "i", "t"].join(""),
        ["v", "u", "l", "n", "e", "r", "a", "b"].join(""),
    ];
    const files = ["README.md", "SECURITY.md", ...expectedDocs.map((name) => join("docs", name))];
    for (const file of files) {
        const content = (
            await readFile(new URL(file.replaceAll("\\", "/"), root), "utf8")
        ).toLowerCase();
        for (const term of blocked) {
            assert.equal(
                new RegExp(`\\b${term}`).test(content),
                false,
                `${file} contains a restricted term`,
            );
        }
    }
});
