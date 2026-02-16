import * as core from "@actions/core";

export interface ActionInputs {
  version: string;
  targets: string[];
  config: string;
  parallel: number | undefined;
  remoteCache: boolean;
  noCache: boolean;
  verbose: boolean;
  workingDirectory: string;
  dryRun: boolean;
  matrixLabel: string;
  matrixContext: Record<string, string>;
}

export function getInputs(): ActionInputs {
  const parallelStr = core.getInput("parallel");
  let parallel: number | undefined;
  if (parallelStr) {
    parallel = parseInt(parallelStr, 10);
    if (isNaN(parallel) || parallel < 1) {
      throw new Error(`Invalid parallel value: ${parallelStr}`);
    }
  }

  const targetsStr = core.getInput("targets").trim();
  const targets = targetsStr ? targetsStr.split(/\s+/) : [];

  // Parse matrix context
  const matrixContextStr = core.getInput("matrix-context").trim();
  let matrixContext: Record<string, string> = {};
  if (matrixContextStr) {
    try {
      const parsed = JSON.parse(matrixContextStr);
      if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
        throw new Error("matrix-context must be a JSON object");
      }
      for (const [key, value] of Object.entries(parsed)) {
        matrixContext[key] = String(value);
      }
    } catch (error) {
      if (error instanceof SyntaxError) {
        throw new Error(`Invalid JSON in matrix-context: ${matrixContextStr}`);
      }
      throw error;
    }
  }

  // Auto-generate matrix label from context values if not explicitly set
  let matrixLabel = core.getInput("matrix-label").trim();
  if (!matrixLabel && Object.keys(matrixContext).length > 0) {
    matrixLabel = Object.values(matrixContext).join(" / ");
  }

  return {
    version: core.getInput("version") || "latest",
    targets,
    config: core.getInput("config") || "dagryn.toml",
    parallel,
    remoteCache: core.getBooleanInput("remote-cache"),
    noCache: core.getBooleanInput("no-cache"),
    verbose: core.getBooleanInput("verbose"),
    workingDirectory: core.getInput("working-directory") || ".",
    dryRun: core.getBooleanInput("dry-run"),
    matrixLabel,
    matrixContext,
  };
}
