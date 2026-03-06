import * as core from "@actions/core";
import { getInputs } from "./inputs";
import { install } from "./installer";
import { runDagryn } from "./runner";
import { writeSummary } from "./summary";

async function run(): Promise<void> {
  try {
    const inputs = getInputs();

    const installedVersion = await install(inputs.version);
    core.info(`Using dagryn v${installedVersion}`);

    const result = await runDagryn(inputs);

    await writeSummary(inputs, result);

    core.setOutput("status", result.exitCode === 0 ? "success" : "failure");
    core.setOutput("duration", result.duration.toString());
    if (inputs.matrixLabel) {
      core.setOutput("matrix-label", inputs.matrixLabel);
    }

    if (result.exitCode !== 0) {
      core.setFailed(`Dagryn run failed with exit code ${result.exitCode}`);
    }
  } catch (error) {
    if (error instanceof Error) {
      core.setFailed(error.message);
    } else {
      core.setFailed("An unexpected error occurred");
    }
  }
}

run();
