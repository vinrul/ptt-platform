import { createServer } from "node:net";

const root = import.meta.dir.replace(/\/scripts$/, "");
const configuredJwtSecret = process.env.JWT_SECRET ?? "";
const postgresUser = process.env.POSTGRES_USER ?? "ptt";
const postgresPassword = process.env.POSTGRES_PASSWORD ?? "ptt";
const postgresDatabase = process.env.POSTGRES_DB ?? "ptt_fleet";
const hostEnvironment = {
  ...process.env,
  APP_ENV: "local",
  DATABASE_URL:
    process.env.LOCAL_DATABASE_URL ??
    `postgres://${encodeURIComponent(postgresUser)}:${encodeURIComponent(postgresPassword)}@localhost:5432/${encodeURIComponent(postgresDatabase)}?sslmode=disable`,
  REDIS_URL: process.env.LOCAL_REDIS_URL ?? "redis://localhost:6379",
  JWT_SECRET:
    configuredJwtSecret.length >= 32
      ? configuredJwtSecret
      : "local-development-jwt-secret-32-bytes",
};
const containerRuntime =
  process.env.CONTAINER_RUNTIME ??
  (Bun.which("docker") ? "docker" : Bun.which("podman") ? "podman" : "");

await main().catch((error: unknown) => {
  console.error(error instanceof Error ? error.message : error);
  process.exit(1);
});

async function main() {
  if (!containerRuntime) {
    throw new Error("Docker or Podman is required for PostgreSQL and Redis.");
  }
  await assertPortAvailable(8080, "API");
  await assertPortAvailable(5173, "dispatcher");

  const compose = [containerRuntime, "compose", "-f", "infra/docker/docker-compose.local.yml"];
  await run([...compose, "up", "-d", "--remove-orphans", "postgres", "redis", "pgweb"]);
  await waitFor([
    ...compose,
    "exec",
    "-T",
    "postgres",
    "pg_isready",
    "-U",
    postgresUser,
    "-d",
    postgresDatabase,
  ]);
  await run(["go", "run", "./cmd/migrate", "up"], "services/api-server");
  await run(["go", "run", "./cmd/seed"], "services/api-server");

  const api = spawn(["go", "run", "./cmd/server"], "services/api-server");
  const web = spawn(["bun", "--filter", "dispatcher-web", "dev"]);
  const stop = () => {
    api.kill();
    web.kill();
  };
  process.on("SIGINT", stop);
  process.on("SIGTERM", stop);

  await Promise.all([
    waitForProcessHttp(api, "http://localhost:8080/readyz", "API"),
    waitForProcessHttp(web, "http://localhost:5173", "dispatcher"),
  ]);
  await run(["bun", "scripts/smoke-local.ts"]);
  console.log("\nLocal stack ready:");
  console.log("- Dispatcher: http://localhost:5173");
  console.log("- API:        http://localhost:8080");
  console.log("- Pgweb:      http://localhost:8081");
  console.log("- Users: admin, dispatcher, field1, field2");
  console.log("- Password: ptt-local-123 (override with SEED_PASSWORD)");
  console.log(`\nPress Ctrl+C to stop API and web. ${containerRuntime} dependencies remain running.`);

  await Promise.all([api.exited, web.exited]);
}

function spawn(command: string[], cwd = ".") {
  return Bun.spawn(command, {
    cwd: `${root}/${cwd}`,
    env: hostEnvironment,
    stdin: "inherit",
    stdout: "inherit",
    stderr: "inherit",
  });
}

async function run(command: string[], cwd = ".") {
  const child = spawn(command, cwd);
  const exitCode = await child.exited;
  if (exitCode !== 0) {
    throw new Error(`Command failed (${exitCode}): ${command.join(" ")}`);
  }
}

async function waitFor(command: string[]) {
  for (let attempt = 0; attempt < 30; attempt += 1) {
    const child = Bun.spawn(command, { cwd: root, stdout: "ignore", stderr: "ignore" });
    if ((await child.exited) === 0) return;
    await Bun.sleep(1_000);
  }
  throw new Error(`Timed out waiting for: ${command.join(" ")}`);
}

async function waitForHttp(url: string) {
  for (let attempt = 0; attempt < 60; attempt += 1) {
    try {
      const response = await fetch(url);
      if (response.ok) return;
    } catch {
      // API is still starting.
    }
    await Bun.sleep(500);
  }
  throw new Error(`Timed out waiting for ${url}`);
}

async function waitForProcessHttp(
  process: ReturnType<typeof spawn>,
  url: string,
  name: string,
) {
  await Promise.race([
    waitForHttp(url),
    process.exited.then((exitCode) => {
      throw new Error(`${name} exited before becoming ready (code ${exitCode})`);
    }),
  ]);
}

async function assertPortAvailable(port: number, name: string) {
  await new Promise<void>((resolve, reject) => {
    const server = createServer();
    server.once("error", () => {
      reject(new Error(`${name} port ${port} is already in use. Stop the existing process first.`));
    });
    server.listen(port, "0.0.0.0", () => {
      server.close(() => resolve());
    });
  });
}
