import { join } from "node:path";

const root = join(import.meta.dir, "..");
const apiDir = join(root, "services", "api-server");
const postgresUser = process.env.POSTGRES_USER ?? "ptt";
const postgresPassword = process.env.POSTGRES_PASSWORD ?? "ptt";
const postgresDatabase = process.env.POSTGRES_DB ?? "ptt_fleet";
const configuredJwtSecret = process.env.JWT_SECRET ?? "";

const environment = {
  ...process.env,
  APP_ENV: process.env.APP_ENV ?? "local",
  DATABASE_URL:
    process.env.LOCAL_DATABASE_URL ??
    `postgres://${encodeURIComponent(postgresUser)}:${encodeURIComponent(postgresPassword)}@localhost:5432/${encodeURIComponent(postgresDatabase)}?sslmode=disable`,
  REDIS_URL: process.env.LOCAL_REDIS_URL ?? "redis://localhost:6379",
  JWT_SECRET:
    configuredJwtSecret.length >= 32
      ? configuredJwtSecret
      : "local-development-jwt-secret-32-bytes",
};

const server = Bun.spawn(["go", "run", "./cmd/server"], {
  cwd: apiDir,
  env: environment,
  stdin: "inherit",
  stdout: "inherit",
  stderr: "inherit",
});

process.on("SIGINT", () => server.kill());
process.on("SIGTERM", () => server.kill());

process.exit(await server.exited);
