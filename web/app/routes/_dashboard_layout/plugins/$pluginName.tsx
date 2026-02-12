import { createFileRoute, useNavigate } from "@tanstack/react-router";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import { Separator } from "~/components/ui/separator";
import { useState } from "react";
import type { PluginInfo } from "~/lib/api";
import { usePlugin } from "~/hooks/queries";
import { Icons } from "~/components/icons";

export const Route = createFileRoute("/_dashboard_layout/plugins/$pluginName")({
  component: PluginDetailPage,
});

function PluginDetailPage() {
  const { pluginName } = Route.useParams();
  const navigate = useNavigate();
  const [copied, setCopied] = useState(false);

  // Fetch plugin details
  const { data, isLoading, error } = usePlugin(pluginName);

  const plugin: PluginInfo | undefined = data;

  const installSnippet = plugin
    ? `[plugins]\n${plugin.name} = "local:./plugins/${plugin.name}"`
    : "";

  const handleCopy = () => {
    navigator.clipboard.writeText(installSnippet);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Icons.Loader className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (error || !plugin) {
    return (
      <Card className="border-destructive">
        <CardHeader>
          <CardTitle className="text-destructive">Error</CardTitle>
          <CardDescription>Plugin not found</CardDescription>
        </CardHeader>
      </Card>
    );
  }

  return (
    <div className="container mx-auto py-8 space-y-6 max-w-4xl px-6">
      <div className="flex items-center space-x-4">
        <Button
          variant="ghost"
          size="icon"
          onClick={() => navigate({ to: "/plugins/browse" })}
        >
          <Icons.ArrowLeft className="h-4 w-4" />
        </Button>
        <div className="flex-1">
          <div className="flex items-center space-x-3">
            <Icons.Package className="h-8 w-8 text-primary" />
            <h1 className="text-3xl font-bold tracking-tight">{plugin.name}</h1>
          </div>
          <p className="text-muted-foreground mt-1">{plugin.description}</p>
        </div>
      </div>

      <div className="flex flex-wrap gap-2">
        <Badge variant="outline">{plugin.type}</Badge>
        <Badge variant="outline">v{plugin.version}</Badge>
        {plugin.author && <Badge variant="secondary">{plugin.author}</Badge>}
        {plugin.license && <Badge variant="outline">{plugin.license}</Badge>}
        {plugin.installed && <Badge variant="default">Installed</Badge>}
      </div>

      <Separator />

      {/* Inputs Section */}
      {plugin.inputs && Object.keys(plugin.inputs).length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Inputs</CardTitle>
            <CardDescription>
              Configuration options for this plugin
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              {Object.entries(plugin.inputs).map(([key, input]) => (
                <div key={key} className="border-l-2 border-primary pl-4">
                  <div className="flex items-center space-x-2">
                    <code className="text-sm font-mono bg-muted px-2 py-1">
                      {key}
                    </code>
                    {input.required && (
                      <Badge variant="destructive" className="text-xs">
                        Required
                      </Badge>
                    )}
                    {input.default && (
                      <Badge variant="outline" className="text-xs">
                        Default: {input.default}
                      </Badge>
                    )}
                  </div>
                  <p className="text-sm text-muted-foreground mt-1">
                    {input.description}
                  </p>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Outputs Section */}
      {plugin.outputs && Object.keys(plugin.outputs).length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Outputs</CardTitle>
            <CardDescription>Values exported by this plugin</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              {Object.entries(plugin.outputs).map(([key, output]) => (
                <div key={key} className="border-l-2 border-green-500 pl-4">
                  <code className="text-sm font-mono bg-muted px-2 py-1">
                    {key}
                  </code>
                  <p className="text-sm text-muted-foreground mt-1">
                    {output.description}
                  </p>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Steps Section (for composite plugins) */}
      {plugin.steps && plugin.steps.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Steps</CardTitle>
            <CardDescription>
              Execution steps for this composite plugin
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              {plugin.steps.map((step, index) => (
                <div
                  key={index}
                  className="border rounded-none p-3 bg-muted/50"
                >
                  <div className="flex items-center space-x-2 mb-2">
                    <Badge variant="outline" className="text-xs">
                      {index + 1}
                    </Badge>
                    <span className="font-medium text-sm">{step.name}</span>
                    {step.if && (
                      <Badge variant="secondary" className="text-xs">
                        Conditional
                      </Badge>
                    )}
                  </div>
                  <pre className="text-xs bg-background p-2 rounded-none overflow-x-auto">
                    <code>{step.command}</code>
                  </pre>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Cleanup Section (for composite plugins with post steps) */}
      {plugin.cleanup && plugin.cleanup.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Cleanup (Post)</CardTitle>
            <CardDescription>
              Steps that run after plugin execution (reverse order)
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              {plugin.cleanup.map((step, index) => (
                <div
                  key={index}
                  className="border rounded-none p-3 bg-muted/50"
                >
                  <div className="flex items-center space-x-2 mb-2">
                    <Badge variant="outline" className="text-xs">
                      {index + 1}
                    </Badge>
                    <span className="font-medium text-sm">{step.name}</span>
                    {step.if && (
                      <Badge variant="secondary" className="text-xs">
                        Conditional
                      </Badge>
                    )}
                  </div>
                  <pre className="text-xs bg-background p-2 rounded overflow-x-auto">
                    <code>{step.command}</code>
                  </pre>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Installation Section */}
      <Card>
        <CardHeader>
          <CardTitle>Installation</CardTitle>
          <CardDescription>Add this plugin to your dagryn.toml</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="relative">
            <pre className="bg-muted p-4 rounded-none overflow-x-auto text-sm">
              <code>{installSnippet}</code>
            </pre>
            <Button
              variant="ghost"
              size="sm"
              className="absolute top-2 right-2"
              onClick={handleCopy}
            >
              {copied ? (
                <>
                  <Icons.Check className="h-4 w-4 mr-2" />
                  Copied
                </>
              ) : (
                <>
                  <Icons.Copy className="h-4 w-4 mr-2" />
                  Copy
                </>
              )}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
