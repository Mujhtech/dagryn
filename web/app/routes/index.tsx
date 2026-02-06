import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { useAuth } from "~/lib/auth";
import { usePackageManagerTab } from "~/hooks/use-url-filters";
import { Button } from "~/components/ui/button";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "~/components/ui/tabs";
import { Copy, Loader2, MessageCircle } from "lucide-react";

export const Route = createFileRoute("/")({
  component: IndexPage,
});

function IndexPage() {
  const navigate = useNavigate();
  const { isAuthenticated, isLoading } = useAuth();
  const { pm, setPm } = usePackageManagerTab();
  const [copiedStep, setCopiedStep] = useState<number | null>(null);

  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      navigate({ to: "/login" });
    }
  }, [isAuthenticated, isLoading, navigate]);

  const copyToClipboard = async (text: string, step: number) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopiedStep(step);
      setTimeout(() => setCopiedStep(null), 2000);
    } catch (err) {
      console.error("Failed to copy:", err);
    }
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (!isAuthenticated) {
    return null;
  }

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
  };

  return (
    <div className="flex flex-1 flex-col">
      <div className="flex-1 flex items-center justify-center p-6">
        <div className="w-full max-w-3xl space-y-8">
          {/* Header */}
          <div className="flex items-center justify-between">
            <h1 className="text-4xl font-bold tracking-tight">
              Get setup in 3 minutes
            </h1>
            <Button variant="ghost" size="sm" asChild>
              <Link to="/projects">
                <MessageCircle className="mr-2 h-4 w-4" />
                I'm stuck!
              </Link>
            </Button>
          </div>

          {/* Step 1 */}
          <div className="space-y-4">
            <h2 className="text-xl font-semibold">
              1. Run the CLI 'init' command in an existing project
            </h2>
            <Tabs value={pm} onValueChange={setPm} className="w-full">
              <TabsList>
                <TabsTrigger value="npm">npm</TabsTrigger>
                <TabsTrigger value="pnpm">pnpm</TabsTrigger>
                <TabsTrigger value="yarn">yarn</TabsTrigger>
              </TabsList>
              <TabsContent value="npm" className="mt-4">
                <CodeBlock
                  command={commands.npm.init}
                  onCopy={() => copyToClipboard(commands.npm.init, 1)}
                  copied={copiedStep === 1}
                />
              </TabsContent>
              <TabsContent value="pnpm" className="mt-4">
                <CodeBlock
                  command={commands.pnpm.init}
                  onCopy={() => copyToClipboard(commands.pnpm.init, 1)}
                  copied={copiedStep === 1}
                />
              </TabsContent>
              <TabsContent value="yarn" className="mt-4">
                <CodeBlock
                  command={commands.yarn.init}
                  onCopy={() => copyToClipboard(commands.yarn.init, 1)}
                  copied={copiedStep === 1}
                />
              </TabsContent>
            </Tabs>
            <p className="text-sm text-muted-foreground">
              You'll notice a new folder in your project called{" "}
              <code className="rounded bg-muted px-1.5 py-0.5 text-xs font-mono">
                .dagryn
              </code>
              . We've added a few simple example tasks in there to help you get
              started.
            </p>
          </div>

          {/* Step 2 */}
          <div className="space-y-4">
            <h2 className="text-xl font-semibold">2. Run your workflow</h2>
            <Tabs value={pm} onValueChange={setPm} className="w-full">
              <TabsList>
                <TabsTrigger value="npm">npm</TabsTrigger>
                <TabsTrigger value="pnpm">pnpm</TabsTrigger>
                <TabsTrigger value="yarn">yarn</TabsTrigger>
              </TabsList>
              <TabsContent value="npm" className="mt-4">
                <CodeBlock
                  command={commands.npm.run}
                  onCopy={() => copyToClipboard(commands.npm.run, 2)}
                  copied={copiedStep === 2}
                />
              </TabsContent>
              <TabsContent value="pnpm" className="mt-4">
                <CodeBlock
                  command={commands.pnpm.run}
                  onCopy={() => copyToClipboard(commands.pnpm.run, 2)}
                  copied={copiedStep === 2}
                />
              </TabsContent>
              <TabsContent value="yarn" className="mt-4">
                <CodeBlock
                  command={commands.yarn.run}
                  onCopy={() => copyToClipboard(commands.yarn.run, 2)}
                  copied={copiedStep === 2}
                />
              </TabsContent>
            </Tabs>
          </div>

          {/* Step 3 */}
          <div className="space-y-4">
            <div className="flex items-center gap-2">
              <h2 className="text-xl font-semibold">3. Waiting for tasks</h2>
              <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
            </div>
            <p className="text-sm text-muted-foreground">
              This page will automatically refresh.
            </p>
          </div>
        </div>
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
          <Copy className="h-4 w-4" />
        )}
      </Button>
    </div>
  );
}
