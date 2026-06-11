import { ZipArchive } from "archiver";
import { createWriteStream } from "node:fs";
import { mkdir, rm } from "node:fs/promises";
import { join } from "node:path";
import { finished } from "node:stream/promises";

const repositoryRoot = join(import.meta.dir, "..");
const artifactRoot = join(repositoryRoot, "ptt-fleet");
const outputDir = join(artifactRoot, "dist-ptt-fleet");
const zipPath = join(artifactRoot, "dist-ptt-fleet.zip");

await mkdir(artifactRoot, { recursive: true });
await rm(outputDir, { recursive: true, force: true });
await rm(zipPath, { force: true });

const child = Bun.spawn(["bun", "--filter", "dispatcher-web", "build"], {
  cwd: repositoryRoot,
  env: {
    ...process.env,
    VITE_API_URL:
      process.env.VITE_API_URL || "https://ptt.vinrul.my.id",
    VITE_WS_URL:
      process.env.VITE_WS_URL || "wss://ptt.vinrul.my.id/ws",
  },
  stdin: "inherit",
  stdout: "inherit",
  stderr: "inherit",
});

const exitCode = await child.exited;
if (exitCode !== 0) {
  process.exit(exitCode);
}

const zipOutput = createWriteStream(zipPath);
const archive = new ZipArchive({ zlib: { level: 9 } });
archive.pipe(zipOutput);
archive.directory(outputDir, "dist-ptt-fleet");

const outputFinished = finished(zipOutput);
await archive.finalize();
await outputFinished;

console.log(`Dispatcher bundle: ${outputDir}`);
console.log(`Dispatcher ZIP: ${zipPath}`);
