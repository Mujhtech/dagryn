import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "~/components/ui/select";
import { api } from "~/lib/api";
import Editor from "@monaco-editor/react";
import React, { useState } from "react";
import { Button } from "~/components/ui/button";
import { Card, CardContent } from "~/components/ui/card";
import { Separator } from "~/components/ui/separator";
import { Icons } from "~/components/icons";
import type { MonacoInstance } from "~/lib/monaco";
import "~/lib/monaco";

const workflowConverterExample = `name: CI
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
      - run: npm ci
      - run: npm test`;

function setupConverterTheme(monaco: MonacoInstance) {
  monaco.editor.defineTheme("dagryn-converter", {
    base: "vs-dark",
    inherit: true,
    rules: [],
    colors: {
      "editor.background": "#0a0a0a",
      "editor.foreground": "#d6e3ff",
      "editorCursor.foreground": "#b9cfff",
      "editorLineNumber.foreground": "#6f83a7",
      "editorLineNumber.activeForeground": "#9ab3dc",
      "editorGutter.background": "#0a0a0a",
      "editor.selectionBackground": "#35518066",
      "editor.inactiveSelectionBackground": "#2b425f66",
      "editorIndentGuide.background1": "#23354f",
      "editorIndentGuide.activeBackground1": "#3a5c8e",
    },
  });
}

function WorkflowConverter() {
  const [workflowYAML, setWorkflowYAML] = useState(workflowConverterExample);
  const [output, setOutput] = useState(
    "# Converted Dagryn tasks will appear here",
  );
  const [source, setSource] = useState("github-action");
  const [isConverting, setIsConverting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [summary, setSummary] = useState<string | null>(null);
  const editorOptions = {
    minimap: { enabled: false },
    wordWrap: "on" as const,
    lineNumbers: "on" as const,
    glyphMargin: false,
    folding: false,
    scrollBeyondLastLine: false,
    renderLineHighlight: "none" as const,
    automaticLayout: true,
    fontSize: 15,
    tabSize: 2,
    insertSpaces: true,
    padding: { top: 10, bottom: 10 },
    scrollbar: {
      verticalScrollbarSize: 8,
      horizontalScrollbarSize: 8,
    },
  };

  async function handleConvert() {
    const trimmed = workflowYAML.trim();
    if (!trimmed) {
      setError("Paste a GitHub Actions workflow YAML first.");
      return;
    }

    setIsConverting(true);
    setError(null);
    setSummary(null);

    try {
      const { data } = await api.translateGitHubWorkflowYAML({
        workflow_yaml: trimmed,
        file_name: "workflow.yml",
      });
      const converted = data.tasks_toml.trim();
      setOutput(
        converted || "# No runnable GitHub Actions steps were detected.",
      );

      if (data.workflows.length > 0) {
        const workflowCount = data.workflows.length;
        const taskCount = data.workflows.reduce(
          (total, workflow) => total + workflow.task_count,
          0,
        );
        setSummary(
          `Converted ${workflowCount} workflow${workflowCount === 1 ? "" : "s"} into ${taskCount} task${taskCount === 1 ? "" : "s"}.`,
        );
      }
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to convert workflow YAML.",
      );
    } finally {
      setIsConverting(false);
    }
  }

  async function handleCopy() {
    if (!output.trim()) return;
    try {
      await navigator.clipboard.writeText(output);
      setCopied(true);
      setTimeout(() => setCopied(false), 1200);
    } catch {
      setError("Unable to copy output.");
    }
  }

  return (
    <Card className="py-0 gap-0 bg-background/35">
      <div className="flex items-center px-4 py-1.5">
        <div className="flex gap-2">
          <Select value={source} onValueChange={setSource}>
            <SelectTrigger className="w-45 h-9">
              <SelectValue placeholder="Select source" />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                <SelectItem value="github-action">
                  <Icons.Github className="h-4 w-4" />
                  <span>GitHub Actions</span>
                </SelectItem>
              </SelectGroup>
            </SelectContent>
          </Select>

          <Button
            size="sm"
            className="h-9"
            onClick={handleConvert}
            disabled={isConverting}
          >
            {isConverting ? (
              <Icons.Loader className="mr-2 h-4 w-4 animate-spin" />
            ) : (
              <Icons.Retry className="mr-2 h-4 w-4" />
            )}
            Convert
          </Button>
        </div>

        <div className="ml-auto">
          <Button
            variant="outline"
            size="sm"
            className="h-9"
            onClick={handleCopy}
          >
            <Icons.Copy className="mr-2 h-4 w-4" />
            {copied ? "Copied" : "Copy"}
          </Button>
        </div>
      </div>
      <Separator />

      <CardContent className="px-0">
        <div className="grid grid-cols-2">
          <div className="flex flex-col">
            <div className="converter-editor-surface">
              <Editor
                height="28rem"
                language="yaml"
                theme="dagryn-converter"
                beforeMount={setupConverterTheme}
                value={workflowYAML}
                onChange={(value) => setWorkflowYAML(value ?? "")}
                options={editorOptions}
              />
            </div>
          </div>

          <div className="flex flex-col border-l border-muted">
            <div className="converter-editor-surface">
              <Editor
                height="28rem"
                language="toml"
                theme="dagryn-converter"
                beforeMount={setupConverterTheme}
                value={output}
                options={{
                  ...editorOptions,
                  readOnly: true,
                }}
              />
            </div>
          </div>
        </div>
      </CardContent>
      {(summary || error) && (
        <>
          <Separator />
          <div className="p-1 space-y-1">
            {summary && (
              <p className="text-sm text-muted-foreground">{summary}</p>
            )}
            {error && <p className="text-sm text-destructive">{error}</p>}
          </div>
        </>
      )}
    </Card>
  );
}

export default React.memo(WorkflowConverter);
