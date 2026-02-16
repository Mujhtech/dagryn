import { createFileRoute, Link } from "@tanstack/react-router";
import { useState } from "react";
import { useAuth } from "~/lib/auth";
import { useDashboardOverview } from "~/hooks/queries";
import { Button } from "~/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { Icons } from "~/components/icons";
import { UsageSummaryCard } from "~/components/dashboard/usage-summary";
import { RecentRunsSection } from "~/components/dashboard/recent-runs";
import { ProjectStatsCard } from "~/components/dashboard/project-stats-card";

export const Route = createFileRoute("/_dashboard_layout/dashboard")({
  component: IndexPage,
});

function IndexPage() {
  const { isAuthenticated } = useAuth();
  const { data: overview, isLoading } = useDashboardOverview(isAuthenticated);
  const [copiedStep, setCopiedStep] = useState<number | null>(null);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Icons.Loader className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  const copyToClipboard = async (text: string, step: number) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopiedStep(step);
      setTimeout(() => setCopiedStep(null), 2000);
    } catch (err) {
      console.error("Failed to copy:", err);
    }
  };

  const commands = {
    npm: {
      init: "npx dagryn@latest init",
      run: "npx dagryn@latest run",
    },
    pnpm: {
      init: "pnpm dlx dagryn@latest init",
      run: "pnpm dlx dagryn@latest run",
    },
    yarn: {
      init: "yarn dlx dagryn@latest init",
      run: "yarn dlx dagryn@latest run",
    },
    install: {
      brew: "brew install mujhtech/tap/dagryn",
      curl: "curl -fsSL https://raw.githubusercontent.com/mujhtech/dagryn/main/install.sh | bash",
      source: "go install github.com/mujhtech/dagryn/cmd/dagryn@latest",
    },
    init: {
      local: "dagryn init",
      remote: "dagryn init --remote",
    },
    run: {
      local: "dagryn run",
      remote: "dagryn run --sync",
      task: "dagryn run <task-name>",
    },
  };

  const projects = overview?.projects ?? [];
  const recentRuns = overview?.recent_runs ?? [];

  const recentProjects = [...projects]
    .sort(
      (a, b) =>
        new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime(),
    )
    .slice(0, 8);

  return (
    <div className="flex flex-1 flex-col p-6">
      <div className="mx-auto w-full space-y-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold tracking-tight">Dashboard</h1>
            <p className="text-muted-foreground">
              Quick overview of all your projects
            </p>
          </div>
          <div className="flex items-center gap-2">
            <Button variant="outline" asChild>
              <Link to="/plugins/browse">
                <Icons.Package className="mr-2 h-4 w-4" />
                Browse Plugins
              </Link>
            </Button>
            <Button asChild>
              <Link to="/projects">
                <Icons.Folder className="mr-2 h-4 w-4" />
                Manage Projects
              </Link>
            </Button>
          </div>
        </div>

        {recentProjects.length === 0 ? (
          <SetupGuide
            commands={commands}
            copiedStep={copiedStep}
            onCopy={copyToClipboard}
          />
        ) : (
          <div className="flex flex-col lg:flex-row gap-6">
            {/* Left: Usage + Recent Runs */}
            <div className="w-full lg:w-85 space-y-6 shrink-0">
              <UsageSummaryCard />
              <RecentRunsSection runs={recentRuns} />
            </div>
            {/* Right: Projects grid */}
            <div className="flex-1 min-w-0">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {recentProjects.map((project) => (
                  <ProjectStatsCard key={project.id} project={project} />
                ))}
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function SetupGuide({
  commands,
  copiedStep,
  onCopy,
}: {
  commands: {
    install: {
      brew: string;
      curl: string;
      source: string;
    };
    init: {
      local: string;
      remote: string;
    };
    run: {
      local: string;
      remote: string;
      task: string;
    };
  };
  copiedStep: number | null;
  onCopy: (text: string, step: number) => void;
}) {
  return (
    <div className="space-y-8 w-full max-w-2xl mx-auto">
      <div className="space-y-4">
        <h2 className="text-xl font-semibold">1. Install Dagryn CLI</h2>
        <Tabs defaultValue="brew" className="w-full">
          <TabsList>
            <TabsTrigger value="brew">brew</TabsTrigger>
            <TabsTrigger value="curl">curl</TabsTrigger>
            <TabsTrigger value="source">source</TabsTrigger>
          </TabsList>
          <TabsContent value="brew" className="mt-4">
            <CodeBlock
              command={commands.install.brew}
              onCopy={() => onCopy(commands.install.brew, 1)}
              copied={copiedStep === 1}
            />
          </TabsContent>
          <TabsContent value="curl" className="mt-4">
            <CodeBlock
              command={commands.install.curl}
              onCopy={() => onCopy(commands.install.curl, 1)}
              copied={copiedStep === 1}
            />
          </TabsContent>
          <TabsContent value="source" className="mt-4">
            <CodeBlock
              command={commands.install.source}
              onCopy={() => onCopy(commands.install.source, 1)}
              copied={copiedStep === 1}
            />
          </TabsContent>
        </Tabs>
        <p className="text-sm text-muted-foreground">
          This creates a{" "}
          <code className="rounded bg-muted px-1.5 py-0.5 text-xs font-mono">
            .dagryn
          </code>{" "}
          folder with starter tasks.
        </p>
      </div>
      <div className="space-y-4">
        <h2 className="text-xl font-semibold">2. Initialize Dagryn</h2>
        <Tabs defaultValue="local" className="w-full">
          <TabsList>
            <TabsTrigger value="local">local</TabsTrigger>
            <TabsTrigger value="remote">remote</TabsTrigger>
          </TabsList>
          <TabsContent value="local" className="mt-4">
            <CodeBlock
              command={commands.init.local}
              onCopy={() => onCopy(commands.init.local, 1)}
              copied={copiedStep === 1}
            />
          </TabsContent>
          <TabsContent value="remote" className="mt-4">
            <CodeBlock
              command={commands.init.remote}
              onCopy={() => onCopy(commands.init.remote, 1)}
              copied={copiedStep === 1}
            />
          </TabsContent>
        </Tabs>
        <p className="text-sm text-muted-foreground">
          This creates a{" "}
          <code className="rounded bg-muted px-1.5 py-0.5 text-xs font-mono">
            .dagryn
          </code>{" "}
          folder with starter tasks.
        </p>
      </div>

      <div className="space-y-4">
        <h2 className="text-xl font-semibold">3. Run your workflow</h2>
        <Tabs defaultValue="local" className="w-full">
          <TabsList>
            <TabsTrigger value="local">local</TabsTrigger>
            <TabsTrigger value="remote">remote</TabsTrigger>
            <TabsTrigger value="task">task</TabsTrigger>
          </TabsList>
          <TabsContent value="local" className="mt-4">
            <CodeBlock
              command={commands.run.local}
              onCopy={() => onCopy(commands.run.local, 2)}
              copied={copiedStep === 2}
            />
          </TabsContent>
          <TabsContent value="remote" className="mt-4">
            <CodeBlock
              command={commands.run.remote}
              onCopy={() => onCopy(commands.run.remote, 2)}
              copied={copiedStep === 2}
            />
          </TabsContent>
          <TabsContent value="task" className="mt-4">
            <CodeBlock
              command={commands.run.task}
              onCopy={() => onCopy(commands.run.task, 2)}
              copied={copiedStep === 2}
            />
          </TabsContent>
        </Tabs>
      </div>

      <div className="rounded-none border border-dashed p-4 text-sm text-muted-foreground">
        Your project will appear here once created in{" "}
        <Link to="/projects" className="underline underline-offset-4">
          Projects
        </Link>
        .
      </div>
    </div>
  );
}

function CodeBlock({
  command,
  onCopy,
  copied,
}: {
  command: string;
  onCopy: () => void;
  copied: boolean;
}) {
  return (
    <div className="relative group">
      <div className="rounded-none bg-muted px-4 py-3 pr-12 font-mono text-sm">
        {command}
      </div>
      <Button
        variant="ghost"
        size="icon"
        className="absolute right-2 top-1/2 -translate-y-1/2 h-8 w-8"
        onClick={onCopy}
        title="Copy to clipboard"
      >
        {copied ? (
          <span className="text-xs text-green-600 dark:text-green-400">
            Copied!
          </span>
        ) : (
          <Icons.Copy className="h-4 w-4" />
        )}
      </Button>
    </div>
  );
}
