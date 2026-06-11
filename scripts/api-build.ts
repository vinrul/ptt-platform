import { cp, mkdir, rm } from "node:fs/promises";
import { join } from "node:path";

const repositoryRoot = join(import.meta.dir, "..");
const serviceDir = join(repositoryRoot, "services", "api-server");
const targetOs = process.env.API_BUILD_GOOS || "linux";
const targetArch = process.env.API_BUILD_GOARCH || "amd64";
const executableSuffix = targetOs === "windows" ? ".exe" : "";
const outputDir = join(repositoryRoot, "ptt-fleet", "api-server");
const goCacheDir = join(repositoryRoot, "ptt-fleet", ".go-build-cache");

await rm(outputDir, { recursive: true, force: true });
await mkdir(outputDir, { recursive: true });
await mkdir(goCacheDir, { recursive: true });

const buildEnvironment = {
  ...process.env,
  CGO_ENABLED: "0",
  GOOS: targetOs,
  GOARCH: targetArch,
  GOCACHE: process.env.GOCACHE || goCacheDir,
};

await runGoBuild(
  join(outputDir, `api-server${executableSuffix}`),
  "./cmd/server",
  buildEnvironment,
);
await runGoBuild(
  join(outputDir, `migrate${executableSuffix}`),
  "./cmd/migrate",
  buildEnvironment,
);
await runGoBuild(
  join(outputDir, `seed${executableSuffix}`),
  "./cmd/seed-superuser",
  buildEnvironment,
);
await cp(join(serviceDir, "migrations"), join(outputDir, "migrations"), {
  recursive: true,
});

console.log(`API production bundle: ${outputDir}`);
console.log(`Target: ${targetOs}/${targetArch}, CGO_ENABLED=0`);

async function runGoBuild(
  outputPath: string,
  packagePath: string,
  environment: Record<string, string | undefined>,
) {
  const child = Bun.spawn(
    ["go", "build", "-trimpath", "-ldflags=-s -w", "-o", outputPath, packagePath],
    {
      cwd: serviceDir,
      env: environment,
      stdin: "inherit",
      stdout: "inherit",
      stderr: "inherit",
    },
  );
  const exitCode = await child.exited;
  if (exitCode !== 0) {
    process.exit(exitCode);
  }
}
