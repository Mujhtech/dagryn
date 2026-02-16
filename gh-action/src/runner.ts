import * as core from "@actions/core";
import * as exec from "@actions/exec";
import type { ActionInputs } from "./inputs";

export interface RunResult {
  exitCode: number;
  duration: number;
}

export async function runDagryn(inputs: ActionInputs): Promise<RunResult> {
  const args: string[] = ["run"];

  args.push("--config", inputs.config);

  if (inputs.parallel !== undefined) {
    args.push("--parallel", inputs.parallel.toString());
  }

  if (inputs.verbose) {
    args.push("--verbose");
  }

  if (inputs.noCache) {
    args.push("--no-cache");
  }

  if (inputs.remoteCache) {
    args.push("--remote-cache");
  }

  if (inputs.dryRun) {
    args.push("--dry-run");
  }

  for (const target of inputs.targets) {
    args.push(target);
  }

  // Build matrix environment variables
  const env: Record<string, string> = { ...process.env as Record<string, string> };
  for (const [key, value] of Object.entries(inputs.matrixContext)) {
    env[`DAGRYN_MATRIX_${key.toUpperCase().replace(/-/g, "_")}`] = value;
  }
  if (inputs.matrixLabel) {
    env["DAGRYN_MATRIX_LABEL"] = inputs.matrixLabel;
  }

  core.info(`Running: dagryn ${args.join(" ")}`);
  if (inputs.matrixLabel) {
    core.info(`Matrix leg: ${inputs.matrixLabel}`);
  }

  const start = Date.now();
  let exitCode = 0;

  try {
    exitCode = await exec.exec("dagryn", args, {
      cwd: inputs.workingDirectory,
      env,
    });
  } catch (error) {
    // exec.exec throws on non-zero exit code
    if (error instanceof Error && "code" in error) {
      exitCode = (error as unknown as { code: number }).code;
    } else {
      exitCode = 1;
    }
  }

  const duration = Math.round((Date.now() - start) / 1000);

  return { exitCode, duration };
}
