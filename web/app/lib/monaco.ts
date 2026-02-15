import { loader } from "@monaco-editor/react";
import * as monaco from "monaco-editor/esm/vs/editor/editor.api.js";
import editorWorker from "monaco-editor/esm/vs/editor/editor.worker.js?worker";
import "monaco-editor/esm/vs/basic-languages/yaml/yaml.contribution.js";
import {
  conf as iniConf,
  language as iniLanguage,
} from "monaco-editor/esm/vs/basic-languages/ini/ini.js";

if (typeof self !== "undefined") {
  (self as typeof globalThis & { MonacoEnvironment?: unknown }).MonacoEnvironment = {
    getWorker: () => new editorWorker(),
  };
}

// Monaco does not ship a built-in TOML grammar. Use INI-like highlighting
// as a pragmatic baseline so we can keep the bundle small.
monaco.languages.register({ id: "toml" });
monaco.languages.setMonarchTokensProvider(
  "toml",
  iniLanguage as any,
);
monaco.languages.setLanguageConfiguration(
  "toml",
  iniConf as any,
);

loader.config({ monaco });

export type MonacoInstance = typeof monaco;
