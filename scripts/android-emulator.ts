const sdkRoot =
  process.env.ANDROID_HOME ??
  process.env.ANDROID_SDK_ROOT ??
  `${process.env.HOME}/Library/Android/sdk`;

const emulator = `${sdkRoot}/emulator/emulator`;
const adb = `${sdkRoot}/platform-tools/adb`;
const avdName = process.env.ANDROID_AVD ?? "Medium_Phone_API_36.1";
const apkPath =
  process.env.ANDROID_APK ??
  "apps/android-kotlin/app/build/outputs/apk/debug/app-debug.apk";
const packageName = "id.nuwiarul.pttfleet";
const activityName = `${packageName}/.MainActivity`;

await main().catch((error: unknown) => {
  console.error(error instanceof Error ? error.message : error);
  process.exit(1);
});

async function main() {
  const command = Bun.argv[2];

  switch (command) {
    case "start":
      await startEmulator();
      break;
    case "install":
      await ensureFile(apkPath);
      await waitForBoot();
      await run(adb, ["install", "-r", apkPath]);
      break;
    case "open":
      await waitForBoot();
      await run(adb, ["shell", "am", "start", "-n", activityName]);
      break;
    case "run":
      await ensureFile(apkPath);
      if (!(await hasRunningDevice())) {
        await startEmulator();
      }
      await waitForBoot();
      await run(adb, ["install", "-r", apkPath]);
      await run(adb, ["shell", "am", "start", "-n", activityName]);
      break;
    default:
      throw new Error("Usage: bun scripts/android-emulator.ts <start|install|open|run>");
  }
}

async function startEmulator() {
  await ensureFile(emulator);

  const avds = await output(emulator, ["-list-avds"]);
  if (!avds.split(/\r?\n/).includes(avdName)) {
    throw new Error(`Android AVD "${avdName}" not found. Available AVDs:\n${avds}`);
  }

  console.log(`Starting Android emulator: ${avdName}`);
  Bun.spawn([emulator, "-avd", avdName], {
    stdin: "ignore",
    stdout: "inherit",
    stderr: "inherit",
  }).unref();
}

async function waitForBoot() {
  await ensureFile(adb);
  console.log("Waiting for Android emulator...");
  await run(adb, ["wait-for-device"]);

  for (let attempt = 0; attempt < 120; attempt += 1) {
    const bootCompleted = (await output(adb, ["shell", "getprop", "sys.boot_completed"])).trim();
    if (bootCompleted === "1") {
      console.log("Android emulator is ready.");
      return;
    }
    await Bun.sleep(1_000);
  }

  throw new Error("Android emulator did not finish booting within 120 seconds.");
}

async function hasRunningDevice() {
  const devices = await output(adb, ["devices"]);
  return devices
    .split(/\r?\n/)
    .slice(1)
    .some((line) => line.trim().endsWith("\tdevice"));
}

async function ensureFile(path: string) {
  if (!(await Bun.file(path).exists())) {
    throw new Error(`Required Android SDK executable not found: ${path}`);
  }
}

async function run(executable: string, args: string[]) {
  const process = Bun.spawn([executable, ...args], {
    stdin: "inherit",
    stdout: "inherit",
    stderr: "inherit",
  });
  const exitCode = await process.exited;
  if (exitCode !== 0) {
    throw new Error(`Command failed with exit code ${exitCode}: ${executable} ${args.join(" ")}`);
  }
}

async function output(executable: string, args: string[]) {
  const process = Bun.spawn([executable, ...args], {
    stdout: "pipe",
    stderr: "pipe",
  });
  const [stdout, stderr, exitCode] = await Promise.all([
    new Response(process.stdout).text(),
    new Response(process.stderr).text(),
    process.exited,
  ]);
  if (exitCode !== 0) {
    throw new Error(stderr.trim() || `Command failed: ${executable} ${args.join(" ")}`);
  }
  return stdout.trim();
}
