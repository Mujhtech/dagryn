import * as core from "@actions/core";
import * as tc from "@actions/tool-cache";
import * as path from "path";

const REPO_OWNER = "mujhtech";
const REPO_NAME = "dagryn";

function getPlatform(): string {
  const platform = process.platform;
  switch (platform) {
    case "linux":
      return "Linux";
    case "darwin":
      return "Darwin";
    case "win32":
      return "Windows";
    default:
      throw new Error(`Unsupported platform: ${platform}`);
  }
}

function getArch(): string {
  const arch = process.arch;
  switch (arch) {
    case "x64":
      return "x86_64";
    case "arm64":
      return "arm64";
    default:
      throw new Error(`Unsupported architecture: ${arch}`);
  }
}

async function resolveVersion(version: string): Promise<string> {
  if (version !== "latest") {
    return version.replace(/^v/, "");
  }

  const url = `https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest`;
  const downloadPath = await tc.downloadTool(url, undefined, undefined, {
    Accept: "application/vnd.github.v3+json",
  });

  const fs = await import("fs");
  const body = fs.readFileSync(downloadPath, "utf8");
  const release = JSON.parse(body);
  const tag: string = release.tag_name;
  return tag.replace(/^v/, "");
}

function getDownloadUrl(version: string, platform: string, arch: string): string {
  const ext = platform === "Windows" ? "zip" : "tar.gz";
  return `https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/v${version}/dagryn_${platform}_${arch}.${ext}`;
}

export async function install(version: string): Promise<string> {
  const resolvedVersion = await resolveVersion(version);
  core.info(`Installing dagryn v${resolvedVersion}`);

  const cached = tc.find("dagryn", resolvedVersion);
  if (cached) {
    core.info(`Found cached dagryn v${resolvedVersion}`);
    core.addPath(cached);
    return resolvedVersion;
  }

  const platform = getPlatform();
  const arch = getArch();
  const url = getDownloadUrl(resolvedVersion, platform, arch);

  core.info(`Downloading dagryn from ${url}`);
  const downloadPath = await tc.downloadTool(url);

  let extractedDir: string;
  if (platform === "Windows") {
    extractedDir = await tc.extractZip(downloadPath);
  } else {
    extractedDir = await tc.extractTar(downloadPath);
  }

  const binName = platform === "Windows" ? "dagryn.exe" : "dagryn";
  const binPath = path.join(extractedDir, binName);

  // Ensure the binary is executable
  if (platform !== "Windows") {
    const fs = await import("fs");
    fs.chmodSync(binPath, 0o755);
  }

  const cachedDir = await tc.cacheDir(extractedDir, "dagryn", resolvedVersion);
  core.addPath(cachedDir);

  core.info(`dagryn v${resolvedVersion} installed successfully`);
  return resolvedVersion;
}
