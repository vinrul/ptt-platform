import { join } from "node:path";

const variant = Bun.argv[2]?.toLowerCase();

if (variant !== "debug" && variant !== "release") {
  throw new Error("Usage: bun scripts/android-build.ts <debug|release>");
}

const projectDir = join(import.meta.dir, "..", "apps", "android-kotlin");
const isWindows = process.platform === "win32";
const gradleWrapper = join(projectDir, isWindows ? "gradlew.bat" : "gradlew");
const tasks =
  variant === "debug"
    ? ["testDebugUnitTest", "assembleDebug"]
    : ["lintRelease", "assembleRelease"];

await run(gradleWrapper, tasks, projectDir);

const apkName = variant === "debug" ? "app-debug.apk" : "app-release.apk";
const apkPath = join(projectDir, "app", "build", "outputs", "apk", variant, apkName);

if (!(await Bun.file(apkPath).exists())) {
  throw new Error(`Build completed but APK was not found: ${apkPath}`);
}

if (variant === "debug") {
  console.log(`Android debug APK: ${apkPath}`);
} else {
  console.log(`Android signed release APK: ${apkPath}`);
}

async function run(executable: string, args: string[], cwd: string) {
  if (!(await Bun.file(executable).exists())) {
    throw new Error(`Required executable not found: ${executable}`);
  }

  const processResult = Bun.spawn([executable, ...args], {
    cwd,
    stdin: "inherit",
    stdout: "inherit",
    stderr: "inherit",
  });
  const exitCode = await processResult.exited;
  if (exitCode !== 0) {
    process.exit(exitCode);
  }
}
