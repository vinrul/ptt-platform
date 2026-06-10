import { mkdir, readdir, rm } from "node:fs/promises";
import { dirname, join } from "node:path";

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

const apkName = variant === "debug" ? "app-debug.apk" : "app-release-unsigned.apk";
const apkPath = join(projectDir, "app", "build", "outputs", "apk", variant, apkName);

if (!(await Bun.file(apkPath).exists())) {
  throw new Error(`Build completed but APK was not found: ${apkPath}`);
}

if (variant === "debug") {
  console.log(`Android debug APK: ${apkPath}`);
} else {
  const signedApkPath = await signLocalRelease(apkPath);
  console.log(`Android release APK for local testing: ${signedApkPath}`);
  console.log("This APK uses the local Android debug key and is not for production distribution.");
}

async function signLocalRelease(unsignedApkPath: string) {
  const sdkRoot =
    process.env.ANDROID_HOME ??
    process.env.ANDROID_SDK_ROOT ??
    (isWindows
      ? join(process.env.LOCALAPPDATA ?? "", "Android", "Sdk")
      : join(process.env.HOME ?? "", "Library", "Android", "sdk"));
  const buildToolsRoot = join(sdkRoot, "build-tools");
  const buildToolsVersions = (await readdir(buildToolsRoot, { withFileTypes: true }))
    .filter((entry) => entry.isDirectory())
    .map((entry) => entry.name)
    .sort(compareVersions)
    .reverse();

  if (buildToolsVersions.length === 0) {
    throw new Error(`No Android build-tools found in ${buildToolsRoot}`);
  }

  const executableSuffix = isWindows ? ".exe" : "";
  const buildToolsDir = join(buildToolsRoot, buildToolsVersions[0]);
  const zipalign = join(buildToolsDir, `zipalign${executableSuffix}`);
  const apksigner = join(buildToolsDir, isWindows ? "apksigner.bat" : "apksigner");
  const userHome = process.env.USERPROFILE ?? process.env.HOME ?? "";
  const keystorePath = join(userHome, ".android", "debug.keystore");
  const keytool = join(
    process.env.JAVA_HOME ?? "",
    "bin",
    isWindows ? "keytool.exe" : "keytool",
  );
  const outputDir = dirname(unsignedApkPath);
  const alignedApkPath = join(outputDir, "app-release-local-aligned.apk");
  const signedApkPath = join(outputDir, "app-release-local.apk");

  await ensureDebugKeystore(keytool, keystorePath);
  await rm(alignedApkPath, { force: true });
  await rm(signedApkPath, { force: true });
  await run(zipalign, ["-p", "-f", "4", unsignedApkPath, alignedApkPath], projectDir);
  await run(
    apksigner,
    [
      "sign",
      "--ks",
      keystorePath,
      "--ks-key-alias",
      "androiddebugkey",
      "--ks-pass",
      "pass:android",
      "--key-pass",
      "pass:android",
      "--out",
      signedApkPath,
      alignedApkPath,
    ],
    projectDir,
  );
  await run(apksigner, ["verify", "--verbose", signedApkPath], projectDir);
  await rm(alignedApkPath, { force: true });

  return signedApkPath;
}

async function ensureDebugKeystore(keytool: string, keystorePath: string) {
  if (await Bun.file(keystorePath).exists()) {
    return;
  }

  await mkdir(dirname(keystorePath), { recursive: true });
  await run(
    keytool,
    [
      "-genkeypair",
      "-keystore",
      keystorePath,
      "-storepass",
      "android",
      "-keypass",
      "android",
      "-alias",
      "androiddebugkey",
      "-keyalg",
      "RSA",
      "-keysize",
      "2048",
      "-validity",
      "10000",
      "-dname",
      "CN=Android Debug,O=Android,C=US",
    ],
    projectDir,
  );
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

function compareVersions(left: string, right: string) {
  const leftParts = left.split(".").map(Number);
  const rightParts = right.split(".").map(Number);
  const length = Math.max(leftParts.length, rightParts.length);

  for (let index = 0; index < length; index += 1) {
    const difference = (leftParts[index] ?? 0) - (rightParts[index] ?? 0);
    if (difference !== 0) {
      return difference;
    }
  }

  return left.localeCompare(right);
}
