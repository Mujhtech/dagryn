declare module "monaco-editor/esm/vs/basic-languages/ini/ini.js" {
  export const conf: unknown;
  export const language: unknown;
}

declare module "monaco-editor/esm/vs/basic-languages/yaml/yaml.contribution.js";

declare module "monaco-editor/esm/vs/editor/editor.worker.js?worker" {
  const MonacoWorker: {
    new (): Worker;
  };
  export default MonacoWorker;
}

