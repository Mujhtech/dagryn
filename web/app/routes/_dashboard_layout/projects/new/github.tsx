import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  useGitHubAppInstallations,
  useGitHubAppRepos,
  useGitHubWorkflowTranslation,
} from "~/hooks/queries";
import { useCreateProject } from "~/hooks/mutations";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Checkbox } from "~/components/ui/checkbox";
import { Input } from "~/components/ui/input";
import { Label } from "~/components/ui/label";
import { Separator } from "~/components/ui/separator";
import { Icons } from "~/components/icons";
import Editor from "@monaco-editor/react";
import type { MonacoInstance } from "~/lib/monaco";
import "~/lib/monaco";
import type { GitHubAppInstallation, GitHubRepo } from "~/lib/api";
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from "~/components/ui/command";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "~/components/ui/popover";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "~/components/ui/collapsible";
import { generateMetadata } from "~/lib/metadata";

export const Route = createFileRoute("/_dashboard_layout/projects/new/github")({
  component: ImportFromGitHubPage,
  head: () => {
    return generateMetadata({ title: "Import from GitHub" });
  },
});

const BASE_INSTALLATION =
  "https://github.com/apps/dagryn-dev/installations/new";

type GitScopeKind =
  | "installation"
  | "linked_account"
  | "manual_url"
  | "gitlab"
  | "bitbucket";

interface GitScope {
  kind: GitScopeKind;
  id?: string;
  installationId?: number;
  installationLogin?: string;
}

interface TriggerConfig {
  enabled: boolean;
  push: boolean;
  pushBranches: string[];
  pullRequest: boolean;
  prBranches: string[];
  prTypes: string[];
}

const DEFAULT_TRIGGER_CONFIG: TriggerConfig = {
  enabled: false,
  push: false,
  pushBranches: [],
  pullRequest: false,
  prBranches: [],
  prTypes: [],
};

const DEFAULT_PR_TYPES = ["opened", "synchronize"];

function setupEditorTheme(monaco: MonacoInstance) {
  monaco.editor.defineTheme("dagryn-import", {
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

const tomlEditorOptions = {
  minimap: { enabled: false },
  wordWrap: "on" as const,
  lineNumbers: "on" as const,
  glyphMargin: false,
  folding: true,
  scrollBeyondLastLine: false,
  renderLineHighlight: "none" as const,
  automaticLayout: true,
  fontSize: 13,
  tabSize: 2,
  insertSpaces: true,
  padding: { top: 8, bottom: 8 },
  scrollbar: {
    verticalScrollbarSize: 8,
    horizontalScrollbarSize: 8,
  },
};

function slugify(text: string): string {
  return text
    .toLowerCase()
    .trim()
    .replace(/[^\w\s-]/g, "-")
    .replace(/[\s_-]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

// Strip any [workflow.trigger*] sections and their content from a TOML config.
function stripTriggerSection(config: string): string {
  const lines = config.split("\n");
  const result: string[] = [];
  let inTrigger = false;

  for (const line of lines) {
    const trimmed = line.trim();

    if (trimmed.startsWith("[workflow.trigger")) {
      inTrigger = true;
      continue;
    }

    if (inTrigger) {
      if (trimmed.startsWith("[") && !trimmed.startsWith("[workflow.trigger")) {
        inTrigger = false;
        result.push(line);
      }
      continue;
    }

    result.push(line);
  }

  return result.join("\n").replace(/\n{3,}/g, "\n\n");
}

// Build the [workflow.trigger] TOML block from a TriggerConfig.
function buildTriggerToml(config: TriggerConfig): string {
  if (!config.enabled) return "";

  const lines: string[] = ["[workflow.trigger]"];

  if (config.push) {
    lines.push("[workflow.trigger.push]");
    if (config.pushBranches.length > 0) {
      const formatted = config.pushBranches.map((b) => `"${b}"`).join(", ");
      lines.push(`branches = [${formatted}]`);
    }
  }

  if (config.pullRequest) {
    lines.push("[workflow.trigger.pull_request]");
    if (config.prBranches.length > 0) {
      const formatted = config.prBranches.map((b) => `"${b}"`).join(", ");
      lines.push(`branches = [${formatted}]`);
    }
    if (config.prTypes.length > 0) {
      const formatted = config.prTypes.map((t) => `"${t}"`).join(", ");
      lines.push(`types = [${formatted}]`);
    }
  }

  return lines.join("\n");
}

// Inject the trigger section into a TOML config, replacing any existing trigger block.
function injectTriggerSection(config: string, trigger: TriggerConfig): string {
  const stripped = stripTriggerSection(config);
  const triggerToml = buildTriggerToml(trigger);

  if (!triggerToml) return stripped;

  // Find insertion point: after [workflow] section's key-value pairs,
  // before the next top-level section (e.g. [plugins], [tasks.*], [cache]).
  const lines = stripped.split("\n");
  let insertIndex = -1;
  let inWorkflow = false;

  for (let i = 0; i < lines.length; i++) {
    const trimmed = lines[i].trim();
    if (trimmed === "[workflow]") {
      inWorkflow = true;
      continue;
    }
    if (inWorkflow && trimmed.startsWith("[")) {
      insertIndex = i;
      break;
    }
  }

  if (insertIndex === -1) {
    return stripped.trimEnd() + "\n\n" + triggerToml + "\n";
  }

  const before = lines.slice(0, insertIndex).join("\n").trimEnd();
  const after = lines.slice(insertIndex).join("\n");
  return before + "\n\n" + triggerToml + "\n\n" + after;
}

function GitScopeSelector({
  installations,
  gitScope,
  onSelect,
}: {
  installations: GitHubAppInstallation[];
  gitScope: GitScope | null;
  onSelect: (scope: GitScope) => void;
}) {
  const [open, setOpen] = useState(false);

  const triggerLabel = gitScope
    ? gitScope.kind === "installation"
      ? gitScope.installationLogin
      : gitScope.kind === "linked_account"
        ? "Linked GitHub account"
        : gitScope.kind === "manual_url"
          ? "Any account (remote URL)"
          : gitScope.kind === "gitlab"
            ? "GitLab"
            : "Bitbucket"
    : "Select a Git namespace...";

  const triggerIcon =
    gitScope?.kind === "installation" || gitScope?.kind === "linked_account"
      ? Icons.Github
      : gitScope?.kind === "manual_url"
        ? Icons.Link2
        : Icons.Globe;

  const TriggerIcon = triggerIcon;

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          role="combobox"
          aria-expanded={open}
          className="w-full justify-between font-normal"
        >
          <span className="flex items-center gap-2 truncate">
            <TriggerIcon className="h-4 w-4 shrink-0" />
            <span className="truncate">{triggerLabel}</span>
          </span>
          <Icons.ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent
        className="w-[var(--radix-popover-trigger-width)] p-0"
        align="start"
      >
        <Command>
          <CommandInput placeholder="Search namespaces..." />
          <CommandList className="scrollbar-foreground scrollbar-track-transparent scrollbar-thin">
            <CommandEmpty>No results found.</CommandEmpty>

            {installations.length > 0 && (
              <CommandGroup heading="GitHub App Installations">
                {installations.map((inst) => (
                  <CommandItem
                    key={inst.id}
                    value={inst.account_login}
                    onSelect={() => {
                      onSelect({
                        kind: "installation",
                        installationId: inst.installation_id,
                        id: inst.id,
                        installationLogin: inst.account_login,
                      });
                      setOpen(false);
                    }}
                  >
                    <Icons.Github className="mr-2 h-4 w-4" />
                    <span className="font-mono">{inst.account_login}</span>
                    <Badge variant="secondary" className="ml-auto text-xs">
                      {inst.account_type}
                    </Badge>
                    {gitScope?.kind === "installation" &&
                      gitScope.id === inst.id && (
                        <Icons.Check className="ml-2 h-4 w-4" />
                      )}
                  </CommandItem>
                ))}
              </CommandGroup>
            )}

            <CommandSeparator />

            <CommandGroup heading="Linked account">
              <CommandItem
                value="linked-github-account"
                onSelect={() => {
                  onSelect({ kind: "linked_account" });
                  setOpen(false);
                }}
              >
                <Icons.Github className="mr-2 h-4 w-4" />
                <span>Linked GitHub account</span>
                {gitScope?.kind === "linked_account" && (
                  <Icons.Check className="ml-auto h-4 w-4" />
                )}
              </CommandItem>
            </CommandGroup>

            <CommandSeparator />

            <CommandGroup heading="Use remote URL">
              <CommandItem
                value="any-account-manual-url"
                onSelect={() => {
                  onSelect({ kind: "manual_url" });
                  setOpen(false);
                }}
              >
                <Icons.Link2 className="mr-2 h-4 w-4" />
                <span>Any account</span>
                {gitScope?.kind === "manual_url" && (
                  <Icons.Check className="ml-auto h-4 w-4" />
                )}
              </CommandItem>
              <CommandItem disabled value="gitlab-coming-soon">
                <Icons.Globe className="mr-2 h-4 w-4" />
                <span>GitLab</span>
                <Badge variant="outline" className="ml-auto text-xs">
                  Coming soon
                </Badge>
              </CommandItem>
              <CommandItem disabled value="bitbucket-coming-soon">
                <Icons.Globe className="mr-2 h-4 w-4" />
                <span>Bitbucket</span>
                <Badge variant="outline" className="ml-auto text-xs">
                  Coming soon
                </Badge>
              </CommandItem>
            </CommandGroup>

            <CommandSeparator />

            <CommandGroup heading="Actions">
              <CommandItem
                value="connect-github-account"
                onSelect={() => {
                  window.open(BASE_INSTALLATION, "_blank");
                }}
              >
                <Icons.Plus className="mr-2 h-4 w-4" />
                <span>Connect GitHub account</span>
              </CommandItem>
              {/* <CommandItem
                value="manage-accounts"
                onSelect={() => {
                  window.open(BASE_INSTALLATION, "_blank");
                }}
              >
                <Icons.Settings className="mr-2 h-4 w-4" />
                <span>Manage accounts</span>
                <Icons.ExternalLink className="ml-auto h-3 w-3 opacity-50" />
              </CommandItem> */}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}

function RepositorySelector({
  repos,
  isLoading,
  selectedRepo,
  onSelect,
  installationId,
}: {
  repos: GitHubRepo[];
  isLoading: boolean;
  selectedRepo: GitHubRepo | null;
  onSelect: (repo: GitHubRepo) => void;
  installationId?: string;
}) {
  const [open, setOpen] = useState(false);

  const manageUrl = installationId
    ? `https://github.com/settings/installations/${installationId}`
    : BASE_INSTALLATION;

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          role="combobox"
          aria-expanded={open}
          className="w-full justify-between font-normal"
          disabled={isLoading}
        >
          {isLoading ? (
            <span className="flex items-center gap-2 text-muted-foreground">
              <Icons.Loader className="h-4 w-4 animate-spin" />
              Loading repositories...
            </span>
          ) : selectedRepo ? (
            <span className="flex items-center gap-2 truncate">
              <Icons.Folder className="h-4 w-4 shrink-0" />
              <span className="truncate font-mono">
                {selectedRepo.full_name}
              </span>
              {selectedRepo.private && (
                <Badge variant="secondary" className="text-xs shrink-0">
                  Private
                </Badge>
              )}
            </span>
          ) : (
            <span className="text-muted-foreground">
              Select a repository...
            </span>
          )}
          <Icons.ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent
        className="w-[var(--radix-popover-trigger-width)] p-0"
        align="start"
      >
        <Command>
          <CommandInput placeholder="Search repositories..." />
          <CommandList>
            <CommandEmpty>No repositories found.</CommandEmpty>
            <CommandGroup>
              {repos.map((repo) => (
                <CommandItem
                  key={repo.id}
                  value={repo.full_name}
                  onSelect={() => {
                    onSelect(repo);
                    setOpen(false);
                  }}
                >
                  <span className="flex items-center gap-2 truncate flex-1 min-w-0">
                    <span className="font-mono truncate">{repo.full_name}</span>
                    {repo.private && (
                      <Icons.Lock className="h-3 w-3 shrink-0 text-muted-foreground" />
                    )}
                  </span>
                  <span className="flex items-center gap-2 shrink-0">
                    <span className="text-xs text-muted-foreground">
                      {repo.default_branch}
                    </span>
                    {selectedRepo?.id === repo.id && (
                      <Icons.Check className="h-4 w-4" />
                    )}
                  </span>
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
          <div className="border-t px-3 py-2">
            <a
              href={manageUrl}
              target="_blank"
              rel="noreferrer"
              className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
            >
              <Icons.Settings className="h-3 w-3" />
              Manage repository access in GitHub settings
              <Icons.ExternalLink className="h-3 w-3" />
            </a>
          </div>
        </Command>
      </PopoverContent>
    </Popover>
  );
}

function ManualUrlInput({
  value,
  onChange,
  disabled,
}: {
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
}) {
  return (
    <div className="grid gap-2">
      <Label htmlFor="manual-repo-url">Repository URL</Label>
      <Input
        id="manual-repo-url"
        placeholder="https://github.com/user/repo.git or git@github.com:user/repo.git"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
        className="font-mono"
      />
      <p className="text-xs text-muted-foreground">
        Enter a SSH or HTTPS clone URL. Workflow detection and run triggers are
        not available for manual URLs.
      </p>
    </div>
  );
}

function BranchTagsInput({
  branches,
  onChange,
  placeholder,
  disabled,
}: {
  branches: string[];
  onChange: (branches: string[]) => void;
  placeholder?: string;
  disabled?: boolean;
}) {
  const [inputValue, setInputValue] = useState("");

  const addBranch = () => {
    const trimmed = inputValue.trim();
    if (trimmed && !branches.includes(trimmed)) {
      onChange([...branches, trimmed]);
    }
    setInputValue("");
  };

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap gap-1.5">
        {branches.map((branch) => (
          <Badge
            key={branch}
            variant="secondary"
            className="gap-1 text-xs font-mono"
          >
            {branch}
            <button
              type="button"
              className="ml-0.5 hover:text-destructive"
              onClick={() => onChange(branches.filter((b) => b !== branch))}
              disabled={disabled}
            >
              <Icons.Close className="h-3 w-3" />
            </button>
          </Badge>
        ))}
      </div>
      <div className="flex gap-2">
        <Input
          value={inputValue}
          onChange={(e) => setInputValue(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              addBranch();
            }
          }}
          placeholder={placeholder ?? "e.g. main"}
          disabled={disabled}
          className="h-8 text-xs font-mono"
        />
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={addBranch}
          disabled={disabled || !inputValue.trim()}
          className="h-8 shrink-0"
        >
          Add
        </Button>
      </div>
    </div>
  );
}

function RunTriggersSection({
  triggerConfig,
  onTriggerChange,
  autoDetected,
  defaultBranch,
  disabled,
}: {
  triggerConfig: TriggerConfig;
  onTriggerChange: (config: TriggerConfig) => void;
  autoDetected: boolean;
  defaultBranch?: string;
  disabled?: boolean;
}) {
  const [isOpen, setIsOpen] = useState(triggerConfig.enabled);
  const enabledCount = [triggerConfig.push, triggerConfig.pullRequest].filter(
    Boolean,
  ).length;

  useEffect(() => {
    if (triggerConfig.enabled) {
      setIsOpen(true);
    }
  }, [triggerConfig.enabled]);

  return (
    <Collapsible open={isOpen} onOpenChange={setIsOpen}>
      <div className="rounded-none bg-card border">
        <CollapsibleTrigger asChild>
          <button
            type="button"
            className="flex w-full items-center justify-between px-4 py-3 text-sm font-medium hover:bg-muted/50 transition-colors"
            disabled={disabled}
          >
            <span className="flex items-center gap-2">
              <Icons.Zap className="h-4 w-4" />
              Run Triggers
              {triggerConfig.enabled && enabledCount > 0 && (
                <Badge variant="secondary" className="text-xs">
                  {enabledCount}
                </Badge>
              )}
              {autoDetected && (
                <Badge variant="outline" className="text-xs">
                  Auto-detected
                </Badge>
              )}
            </span>
            {isOpen ? (
              <Icons.ChevronDown className="h-4 w-4" />
            ) : (
              <Icons.ChevronRight className="h-4 w-4" />
            )}
          </button>
        </CollapsibleTrigger>

        <CollapsibleContent>
          <Separator />
          <div className="px-4 py-3 space-y-4">
            <div className="flex items-center gap-2">
              <Checkbox
                id="trigger-enabled"
                checked={triggerConfig.enabled}
                onCheckedChange={(checked) =>
                  onTriggerChange({
                    ...triggerConfig,
                    enabled: Boolean(checked),
                  })
                }
                disabled={disabled}
              />
              <Label htmlFor="trigger-enabled" className="text-sm font-medium">
                Trigger runs when...
              </Label>
            </div>

            {triggerConfig.enabled && (
              <div className="ml-6 space-y-4">
                {/* Push trigger */}
                <div className="space-y-2">
                  <div className="flex items-center gap-2">
                    <Checkbox
                      id="trigger-push"
                      checked={triggerConfig.push}
                      onCheckedChange={(checked) => {
                        const pushEnabled = Boolean(checked);
                        onTriggerChange({
                          ...triggerConfig,
                          push: pushEnabled,
                          pushBranches:
                            pushEnabled &&
                            triggerConfig.pushBranches.length === 0 &&
                            defaultBranch
                              ? [defaultBranch]
                              : triggerConfig.pushBranches,
                        });
                      }}
                      disabled={disabled}
                    />
                    <Label htmlFor="trigger-push" className="text-sm">
                      Code is pushed to a branch
                    </Label>
                  </div>
                  {triggerConfig.push && (
                    <div className="ml-6 space-y-1.5">
                      <Label className="text-xs text-muted-foreground">
                        Branches
                      </Label>
                      <BranchTagsInput
                        branches={triggerConfig.pushBranches}
                        onChange={(pushBranches) =>
                          onTriggerChange({ ...triggerConfig, pushBranches })
                        }
                        placeholder="e.g. main, develop"
                        disabled={disabled}
                      />
                    </div>
                  )}
                </div>

                {/* Pull request trigger */}
                <div className="space-y-2">
                  <div className="flex items-center gap-2">
                    <Checkbox
                      id="trigger-pr"
                      checked={triggerConfig.pullRequest}
                      onCheckedChange={(checked) => {
                        const prEnabled = Boolean(checked);
                        onTriggerChange({
                          ...triggerConfig,
                          pullRequest: prEnabled,
                          prBranches:
                            prEnabled &&
                            triggerConfig.prBranches.length === 0 &&
                            defaultBranch
                              ? [defaultBranch]
                              : triggerConfig.prBranches,
                          prTypes:
                            prEnabled && triggerConfig.prTypes.length === 0
                              ? [...DEFAULT_PR_TYPES]
                              : triggerConfig.prTypes,
                        });
                      }}
                      disabled={disabled}
                    />
                    <Label htmlFor="trigger-pr" className="text-sm">
                      Pull Request is opened
                    </Label>
                  </div>
                  {triggerConfig.pullRequest && (
                    <div className="ml-6 space-y-3">
                      <div className="space-y-1.5">
                        <Label className="text-xs text-muted-foreground">
                          Target branches
                        </Label>
                        <BranchTagsInput
                          branches={triggerConfig.prBranches}
                          onChange={(prBranches) =>
                            onTriggerChange({ ...triggerConfig, prBranches })
                          }
                          placeholder="e.g. main, develop"
                          disabled={disabled}
                        />
                      </div>
                      <div className="space-y-1.5">
                        <Label className="text-xs text-muted-foreground">
                          Event types
                        </Label>
                        <BranchTagsInput
                          branches={triggerConfig.prTypes}
                          onChange={(prTypes) =>
                            onTriggerChange({ ...triggerConfig, prTypes })
                          }
                          placeholder="e.g. opened, synchronize"
                          disabled={disabled}
                        />
                      </div>
                    </div>
                  )}
                </div>
              </div>
            )}

            <p className="text-xs text-muted-foreground">
              Run triggers are configured in your dagryn.toml file.
            </p>
          </div>
        </CollapsibleContent>
      </div>
    </Collapsible>
  );
}

function WorkflowDetectionSection({
  workflowTranslation,
  workflowTranslationLoading,
  workflowTranslationError,
  useDetectedWorkflow,
  setUseDetectedWorkflow,
  workflowDraft,
  setWorkflowDraft,
  repoLanguage,
}: {
  workflowTranslation:
    | {
        detected: boolean;
        workflows: { file: string; name: string; task_count: number }[];
        tasks_toml: string;
      }
    | null
    | undefined;
  workflowTranslationLoading: boolean;
  workflowTranslationError: Error | null;
  useDetectedWorkflow: boolean;
  setUseDetectedWorkflow: (v: boolean) => void;
  workflowDraft: string;
  setWorkflowDraft: (v: string) => void;
  repoLanguage?: string;
}) {
  return (
    <div className="grid gap-2">
      <Label>GitHub workflow detection</Label>
      {workflowTranslationLoading ? (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Icons.Loader className="h-4 w-4 animate-spin" />
          Checking .github/workflows...
        </div>
      ) : workflowTranslationError ? (
        <p className="text-sm text-destructive">Failed to inspect workflows.</p>
      ) : workflowTranslation?.detected ? (
        <div className="rounded-none border bg-muted/40 p-3 space-y-3">
          <div className="flex items-center gap-2">
            <Checkbox
              id="use-detected-workflow"
              checked={useDetectedWorkflow}
              onCheckedChange={(checked) =>
                setUseDetectedWorkflow(Boolean(checked))
              }
            />
            <Label htmlFor="use-detected-workflow" className="text-sm">
              Use detected workflow (auto-sync after create)
            </Label>
          </div>

          <p className="text-xs text-muted-foreground">
            Found {workflowTranslation.workflows.length} workflow
            {workflowTranslation.workflows.length === 1 ? "" : "s"}. You can
            edit the generated configuration before creating the project.
          </p>

          {useDetectedWorkflow && (
            <div className="rounded-none border overflow-hidden">
              <Editor
                height="320px"
                language="toml"
                theme="dagryn-import"
                beforeMount={setupEditorTheme}
                value={workflowDraft}
                onChange={(value) => setWorkflowDraft(value ?? "")}
                options={tomlEditorOptions}
              />
            </div>
          )}
        </div>
      ) : (
        <div className="space-y-3">
          <p className="text-sm text-muted-foreground">
            No workflow configuration found in the selected repository. You can
            start with a sample configuration below.
          </p>
          <div className="rounded-none border bg-muted/40 p-3 space-y-3">
            <div className="flex items-center gap-2">
              <Checkbox
                id="use-sample-workflow"
                checked={useDetectedWorkflow}
                onCheckedChange={(checked) =>
                  setUseDetectedWorkflow(Boolean(checked))
                }
              />
              <Label htmlFor="use-sample-workflow" className="text-sm">
                Use sample configuration (sync after create)
              </Label>
            </div>

            <p className="text-xs text-muted-foreground">
              {repoLanguage
                ? `Sample config for ${repoLanguage} projects. Edit to match your setup.`
                : "Edit the sample tasks to match your project setup."}
            </p>

            {useDetectedWorkflow && (
              <div className="rounded-none border overflow-hidden">
                <Editor
                  height="320px"
                  language="toml"
                  theme="dagryn-import"
                  beforeMount={setupEditorTheme}
                  value={workflowDraft}
                  onChange={(value) => setWorkflowDraft(value ?? "")}
                  options={tomlEditorOptions}
                />
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

function ProjectDetailsSection({
  importName,
  setImportName,
  importSlug,
  setImportSlug,
  setImportSlugEdited,
  branch,
  onBranchChange,
  disabled,
}: {
  importName: string;
  setImportName: (v: string) => void;
  importSlug: string;
  setImportSlug: (v: string) => void;
  setImportSlugEdited: (v: boolean) => void;
  branch: string;
  onBranchChange: (v: string) => void;
  disabled?: boolean;
}) {
  return (
    <div className="space-y-4">
      <div className="grid gap-2">
        <Label htmlFor="import-name">Project name</Label>
        <Input
          id="import-name"
          value={importName}
          onChange={(e) => setImportName(e.target.value)}
          placeholder="My Project"
          disabled={disabled}
        />
      </div>

      <div className="grid gap-2">
        <Label htmlFor="import-slug">Slug</Label>
        <Input
          id="import-slug"
          value={importSlug}
          onChange={(e) => {
            setImportSlug(e.target.value);
            setImportSlugEdited(true);
          }}
          placeholder="my-project"
          disabled={disabled}
          className="font-mono"
        />
      </div>

      <div className="grid gap-2">
        <Label htmlFor="default-branch">Default branch</Label>
        <div className="flex items-center gap-2">
          <Icons.GitBranch className="h-4 w-4 shrink-0 text-muted-foreground" />
          <Input
            id="default-branch"
            value={branch}
            onChange={(e) => onBranchChange(e.target.value)}
            placeholder="main"
            disabled={disabled}
            className="font-mono"
          />
        </div>
        <p className="text-xs text-muted-foreground">
          Change the branch to detect workflows from a different branch.
        </p>
      </div>
    </div>
  );
}

function ImportFromGitHubPage() {
  const navigate = useNavigate();
  const createProjectMutation = useCreateProject();

  const [gitScope, setGitScope] = useState<GitScope | null>(null);
  const [selectedRepo, setSelectedRepo] = useState<GitHubRepo | null>(null);
  const [manualRepoUrl, setManualRepoUrl] = useState("");
  const [triggerConfig, setTriggerConfig] = useState<TriggerConfig>({
    ...DEFAULT_TRIGGER_CONFIG,
  });
  const [branchOverride, setBranchOverride] = useState("");

  const [importName, setImportName] = useState("");
  const [importSlug, setImportSlug] = useState("");
  const [importSlugEdited, setImportSlugEdited] = useState(false);

  const [useDetectedWorkflow, setUseDetectedWorkflow] = useState(true);
  const [workflowDraft, setWorkflowDraft] = useState("");
  const [workflowSyncError, setWorkflowSyncError] = useState("");
  const [pendingProjectId, setPendingProjectId] = useState<string | null>(null);

  // The effective branch: user override or repo default
  const effectiveBranch = branchOverride || selectedRepo?.default_branch || "";
  // Only pass ref when user has overridden (different from default)
  const refParam =
    branchOverride && branchOverride !== selectedRepo?.default_branch
      ? branchOverride
      : undefined;

  const { data: installations = [], isLoading: installationsLoading } =
    useGitHubAppInstallations();

  const { data: appRepos = [], isLoading: appReposLoading } = useGitHubAppRepos(
    gitScope?.kind === "installation" ? (gitScope.id ?? null) : null,
  );

  const {
    data: oauthRepos = [],
    isLoading: oauthReposLoading,
    error: oauthReposError,
  } = useQuery({
    queryKey: queryKeys.githubRepos,
    queryFn: async () => {
      const response = await api.listGitHubRepos();
      return response.data;
    },
    enabled: gitScope?.kind === "linked_account",
    retry: false,
  });

  const activeRepos = gitScope?.kind === "installation" ? appRepos : oauthRepos;
  const reposLoading =
    gitScope?.kind === "installation" ? appReposLoading : oauthReposLoading;

  const {
    data: workflowTranslation,
    isLoading: workflowTranslationLoading,
    error: workflowTranslationError,
  } = useGitHubWorkflowTranslation(
    selectedRepo ? selectedRepo.full_name : null,
    gitScope?.kind === "installation" ? (gitScope.id ?? null) : null,
    refParam,
  );

  // Fetch sample template when no workflow is detected
  const needsSampleTemplate =
    !!selectedRepo &&
    !!workflowTranslation &&
    !workflowTranslation.detected &&
    !!selectedRepo.language;

  const { data: sampleTemplateData } = useQuery({
    queryKey: queryKeys.sampleTemplate(selectedRepo?.language ?? ""),
    queryFn: async () => {
      const res = await api.getSampleTemplate(selectedRepo!.language);
      return res.data;
    },
    enabled: needsSampleTemplate,
  });

  // Changing git scope → reset everything downstream
  const handleScopeChange = (scope: GitScope) => {
    setGitScope(scope);
    setSelectedRepo(null);
    setManualRepoUrl("");
    setBranchOverride("");
    setTriggerConfig({ ...DEFAULT_TRIGGER_CONFIG });
    setImportName("");
    setImportSlug("");
    setImportSlugEdited(false);
    setWorkflowDraft("");
    setUseDetectedWorkflow(true);
    setWorkflowSyncError("");
    setPendingProjectId(null);
  };

  // Changing repo → reset project form + workflow
  const handleRepoSelect = (repo: GitHubRepo) => {
    setSelectedRepo(repo);
    setBranchOverride("");
    setImportName(repo.full_name);
    setImportSlug(slugify(repo.full_name));
    setImportSlugEdited(false);
    setTriggerConfig({ ...DEFAULT_TRIGGER_CONFIG });
    setWorkflowDraft("");
    setUseDetectedWorkflow(true);
    setWorkflowSyncError("");
    setPendingProjectId(null);
  };

  // Auto-populate project details from manual URL
  useEffect(() => {
    if (gitScope?.kind === "manual_url" && manualRepoUrl && !importSlugEdited) {
      const match = manualRepoUrl.match(
        /(?:github\.com|gitlab\.com|bitbucket\.org)[/:](.+?)(?:\.git)?$/,
      );
      if (match) {
        setImportName(match[1]);
        setImportSlug(slugify(match[1]));
      }
    }
  }, [manualRepoUrl, gitScope?.kind, importSlugEdited]);

  // Auto-detect triggers from workflow translation
  useEffect(() => {
    if (!selectedRepo) {
      setWorkflowDraft("");
      setUseDetectedWorkflow(false);
      setWorkflowSyncError("");
      setPendingProjectId(null);
      return;
    }

    if (workflowTranslation?.detected) {
      setWorkflowDraft(workflowTranslation.tasks_toml.trim());
      setUseDetectedWorkflow(true);
      // Auto-enable push trigger when workflows are detected
      setTriggerConfig({
        ...DEFAULT_TRIGGER_CONFIG,
        enabled: true,
        push: true,
        pushBranches: effectiveBranch ? [effectiveBranch] : [],
      });
    } else if (workflowTranslation && !workflowTranslation.detected) {
      // No workflows found — populate with sample config from backend template
      setWorkflowDraft(sampleTemplateData?.template ?? "");
      setUseDetectedWorkflow(false);
    } else {
      setWorkflowDraft("");
      setUseDetectedWorkflow(false);
    }

    setWorkflowSyncError("");
    setPendingProjectId(null);
  }, [selectedRepo, workflowTranslation, effectiveBranch, sampleTemplateData]);

  // Inject trigger config into editor whenever triggers change
  useEffect(() => {
    setWorkflowDraft((prev) => {
      if (!prev) return prev;
      return injectTriggerSection(prev, triggerConfig);
    });
  }, [triggerConfig]);

  const canCreate =
    gitScope?.kind === "manual_url"
      ? manualRepoUrl.trim().length > 0 &&
        importName.trim().length > 0 &&
        importSlug.trim().length > 0
      : selectedRepo !== null &&
        importName.trim().length > 0 &&
        importSlug.trim().length > 0;

  const handleCreate = async () => {
    if (!canCreate) return;
    setWorkflowSyncError("");

    const repoUrl =
      gitScope?.kind === "manual_url"
        ? manualRepoUrl.trim()
        : selectedRepo!.clone_url;

    try {
      const project = await createProjectMutation.mutateAsync({
        name: importName.trim(),
        slug: importSlug.trim(),
        repo_url: repoUrl,
        github_installation_id:
          gitScope?.kind === "installation" ? (gitScope.id ?? "") : "",
        github_repo_id:
          gitScope?.kind !== "manual_url" ? selectedRepo?.id : undefined,
        visibility: "private",
        default_branch: effectiveBranch || undefined,
        dagryn_config:
          useDetectedWorkflow && workflowDraft.trim()
            ? workflowDraft.trim()
            : undefined,
      });

      navigate({
        to: "/projects/$projectId",
        params: { projectId: project.id },
      });
    } catch (err) {
      setWorkflowSyncError(
        err instanceof Error ? err.message : "Failed to create project.",
      );
    }
  };

  const needsGitHubLogin =
    gitScope?.kind === "linked_account" &&
    oauthReposError &&
    "status" in oauthReposError &&
    (oauthReposError as { status?: number }).status === 403;

  if (installationsLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Icons.Loader className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  const showRepoSelector =
    gitScope?.kind === "installation" || gitScope?.kind === "linked_account";
  const showManualUrl = gitScope?.kind === "manual_url";
  const hasSelectedSource =
    gitScope?.kind === "manual_url"
      ? manualRepoUrl.trim().length > 0
      : selectedRepo !== null;

  return (
    <div className="space-y-6 px-6 @container/main py-3">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">
            New Project from Git
          </h1>
          <p className="text-muted-foreground">
            Import a repository to create a new project
          </p>
        </div>
        <Button variant="outline" asChild>
          <Link to="/projects">
            <Icons.ArrowLeft className="mr-2 h-4 w-4" />
            Back to Projects
          </Link>
        </Button>
      </div>

      {/* Git Scope */}
      <Card>
        <CardHeader>
          <CardTitle>Git Scope</CardTitle>
          <CardDescription>
            Choose how you want to connect your repository
          </CardDescription>
        </CardHeader>
        <CardContent>
          <GitScopeSelector
            installations={installations}
            gitScope={gitScope}
            onSelect={handleScopeChange}
          />
        </CardContent>
      </Card>

      {/* Repository / Manual URL */}
      {gitScope && (
        <Card>
          <CardHeader>
            <CardTitle>Repository</CardTitle>
            <CardDescription>
              {showRepoSelector
                ? "Select a repository from your account"
                : "Enter the clone URL for your repository"}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {needsGitHubLogin && (
              <div className="rounded-none border border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-950/30 p-4 text-sm">
                <p className="mb-2">
                  No GitHub account linked. Log in with GitHub to import
                  repositories.
                </p>
                <Button asChild variant="outline" size="sm">
                  <Link to="/login">Connect GitHub account</Link>
                </Button>
              </div>
            )}

            {showRepoSelector && !needsGitHubLogin && (
              <RepositorySelector
                repos={activeRepos}
                isLoading={reposLoading}
                selectedRepo={selectedRepo}
                onSelect={handleRepoSelect}
                installationId={
                  gitScope.kind === "installation"
                    ? gitScope.installationId?.toString()
                    : undefined
                }
              />
            )}

            {showManualUrl && (
              <ManualUrlInput
                value={manualRepoUrl}
                onChange={setManualRepoUrl}
                disabled={createProjectMutation.isPending}
              />
            )}
          </CardContent>
        </Card>
      )}

      {/* Run Triggers — only for non-manual flows with a selected repo */}
      {hasSelectedSource && gitScope?.kind !== "manual_url" && (
        <RunTriggersSection
          triggerConfig={triggerConfig}
          onTriggerChange={setTriggerConfig}
          autoDetected={Boolean(workflowTranslation?.detected)}
          defaultBranch={selectedRepo?.default_branch}
          disabled={createProjectMutation.isPending}
        />
      )}

      {/* Workflow Detection — only for non-manual flows with a selected repo */}
      {selectedRepo && gitScope?.kind !== "manual_url" && (
        <Card>
          <CardHeader>
            <CardTitle>Workflow Detection</CardTitle>
            <CardDescription>
              Automatically translate GitHub Actions workflows to Dagryn
              configuration
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <WorkflowDetectionSection
              workflowTranslation={workflowTranslation}
              workflowTranslationLoading={workflowTranslationLoading}
              workflowTranslationError={workflowTranslationError}
              useDetectedWorkflow={useDetectedWorkflow}
              setUseDetectedWorkflow={setUseDetectedWorkflow}
              workflowDraft={workflowDraft}
              setWorkflowDraft={setWorkflowDraft}
              repoLanguage={selectedRepo?.language}
            />
          </CardContent>
        </Card>
      )}

      {/* Project Details */}
      {hasSelectedSource && (
        <Card>
          <CardHeader>
            <CardTitle>Project Details</CardTitle>
            <CardDescription>
              Configure the name and slug for your new project
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <ProjectDetailsSection
              importName={importName}
              setImportName={setImportName}
              importSlug={importSlug}
              setImportSlug={setImportSlug}
              setImportSlugEdited={setImportSlugEdited}
              branch={effectiveBranch}
              onBranchChange={setBranchOverride}
              disabled={createProjectMutation.isPending}
            />

            {/* Errors */}
            {workflowSyncError && (
              <div className="rounded-none bg-destructive/10 p-3 text-sm text-destructive">
                <p>{workflowSyncError}</p>
                {pendingProjectId && (
                  <div className="mt-2">
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() =>
                        navigate({
                          to: "/projects/$projectId",
                          params: { projectId: pendingProjectId },
                        })
                      }
                    >
                      Continue anyway
                    </Button>
                  </div>
                )}
              </div>
            )}

            {createProjectMutation.error && (
              <div className="rounded-none bg-destructive/10 p-3 text-sm text-destructive">
                {createProjectMutation.error.message}
              </div>
            )}

            {/* Actions */}
            <div className="flex items-center justify-end gap-2 pt-2">
              <Button
                onClick={handleCreate}
                disabled={!canCreate || createProjectMutation.isPending}
              >
                {createProjectMutation.isPending ? (
                  <>
                    <Icons.Loader className="mr-2 h-4 w-4 animate-spin" />
                    Creating...
                  </>
                ) : (
                  <>
                    <Icons.Plus className="mr-2 h-4 w-4" />
                    Create Project
                  </>
                )}
              </Button>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
