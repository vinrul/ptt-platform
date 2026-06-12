import { basename, join, resolve } from "node:path";

const projectDir = join(import.meta.dir, "..", "apps", "android-kotlin");
const isWindows = process.platform === "win32";
const gradleWrapper = join(projectDir, isWindows ? "gradlew.bat" : "gradlew");
const localPropertiesPath = join(projectDir, "local.properties");
const aabPath = join(
  projectDir,
  "app",
  "build",
  "outputs",
  "bundle",
  "release",
  "app-release.aab",
);

const requiredSigningVariables = [
  "ANDROID_RELEASE_STORE_FILE",
  "ANDROID_RELEASE_STORE_PASSWORD",
  "ANDROID_RELEASE_KEY_ALIAS",
  "ANDROID_RELEASE_KEY_PASSWORD",
] as const;

const signingProperties = await readLocalProperties(localPropertiesPath);
const signingValues = {
  ANDROID_RELEASE_STORE_FILE: signingValue(
    "ANDROID_RELEASE_STORE_FILE",
    "release.storeFile",
  ),
  ANDROID_RELEASE_STORE_PASSWORD: signingValue(
    "ANDROID_RELEASE_STORE_PASSWORD",
    "release.storePassword",
  ),
  ANDROID_RELEASE_KEY_ALIAS: signingValue(
    "ANDROID_RELEASE_KEY_ALIAS",
    "release.keyAlias",
  ),
  ANDROID_RELEASE_KEY_PASSWORD: signingValue(
    "ANDROID_RELEASE_KEY_PASSWORD",
    "release.keyPassword",
  ),
};

const missingVariables = requiredSigningVariables.filter(
  (name) => !signingValues[name],
);

if (missingVariables.length > 0) {
  throw new Error(
    [
      "Play Store release bundle needs release signing configuration.",
      `Missing environment variables: ${missingVariables.join(", ")}`,
      "Set them in your shell or in apps/android-kotlin/local.properties before running this script.",
    ].join("\n"),
  );
}

const storeFile = await resolveExistingStoreFile(
  signingValues.ANDROID_RELEASE_STORE_FILE!,
);
if (!(await Bun.file(storeFile).exists())) {
  throw new Error(`Release keystore was not found: ${storeFile}`);
}

process.env.ANDROID_RELEASE_STORE_FILE = storeFile;
process.env.ANDROID_RELEASE_STORE_PASSWORD =
  signingValues.ANDROID_RELEASE_STORE_PASSWORD!;
process.env.ANDROID_RELEASE_KEY_ALIAS = signingValues.ANDROID_RELEASE_KEY_ALIAS!;
process.env.ANDROID_RELEASE_KEY_PASSWORD =
  signingValues.ANDROID_RELEASE_KEY_PASSWORD!;

console.log(`Using release keystore: ${basename(storeFile)}`);
await run(gradleWrapper, ["lintRelease", "bundleRelease"], projectDir);

if (!(await Bun.file(aabPath).exists())) {
  throw new Error(`Build completed but AAB was not found: ${aabPath}`);
}

console.log(`Android Play Store release bundle: ${aabPath}`);

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

function signingValue(environmentName: string, propertyName: string): string | undefined {
  return (
    process.env[environmentName]?.trim() ||
    signingProperties.get(propertyName)?.trim() ||
    undefined
  );
}

async function readLocalProperties(path: string) {
  const properties = new Map<string, string>();
  const file = Bun.file(path);
  if (!(await file.exists())) {
    return properties;
  }

  const content = await file.text();
  for (const line of content.split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith("#") || trimmed.startsWith("!")) {
      continue;
    }

    const separatorIndex = findPropertySeparator(trimmed);
    if (separatorIndex === -1) {
      continue;
    }

    const key = unescapePropertyValue(trimmed.slice(0, separatorIndex).trim());
    const value = unescapePropertyValue(trimmed.slice(separatorIndex + 1).trim());
    if (key) {
      properties.set(key, value);
    }
  }

  return properties;
}

function findPropertySeparator(line: string) {
  const equalsIndex = line.indexOf("=");
  const colonIndex = line.indexOf(":");
  if (equalsIndex !== -1) {
    return equalsIndex;
  }
  return colonIndex;
}

async function resolveExistingStoreFile(path: string) {
  const candidates = [
    resolve(path),
    resolve(projectDir, path),
    resolve(projectDir, "app", path),
  ];

  for (const candidate of candidates) {
    if (await Bun.file(candidate).exists()) {
      return candidate;
    }
  }

  return candidates[0];
}

function unescapePropertyValue(value: string) {
  let result = "";
  for (let index = 0; index < value.length; index += 1) {
    const character = value[index];
    if (character !== "\\" || index === value.length - 1) {
      result += character;
      continue;
    }

    const nextCharacter = value[index + 1];
    if ("\\:=#! \t\r\n".includes(nextCharacter)) {
      result += nextCharacter;
      index += 1;
      continue;
    }

    result += character;
  }

  return result;
}
