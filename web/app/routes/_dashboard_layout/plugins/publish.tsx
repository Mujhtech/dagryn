import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Textarea } from "~/components/ui/textarea";
import { Label } from "~/components/ui/label";
import { Icons } from "~/components/icons";
import { usePublishPlugin } from "~/hooks/mutations";

export const Route = createFileRoute("/_dashboard_layout/plugins/publish")({
  component: PublishPluginPage,
});

function PublishPluginPage() {
  const navigate = useNavigate();
  const publishMutation = usePublishPlugin();

  const [publisher, setPublisher] = useState("");
  const [pluginName, setPluginName] = useState("");
  const [version, setVersion] = useState("");
  const [manifestJson, setManifestJson] = useState(
    JSON.stringify(
      {
        plugin: {
          name: "",
          type: "composite",
          version: "",
          author: "",
          description: "",
        },
        step: [{ name: "", command: "" }],
      },
      null,
      2,
    ),
  );
  const [releaseNotes, setReleaseNotes] = useState("");
  const [parseError, setParseError] = useState<string | null>(null);

  const handleManifestChange = (value: string) => {
    setManifestJson(value);
    setParseError(null);
    try {
      const parsed = JSON.parse(value);
      if (parsed.plugin) {
        if (parsed.plugin.name && typeof parsed.plugin.name === "string") {
          setPluginName(parsed.plugin.name);
        }
        if (parsed.plugin.version && typeof parsed.plugin.version === "string") {
          setVersion(parsed.plugin.version);
        }
      }
    } catch {
      // Don't show error while typing
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setParseError(null);

    let manifest: Record<string, unknown>;
    try {
      manifest = JSON.parse(manifestJson);
    } catch (err) {
      setParseError(
        err instanceof Error ? err.message : "Invalid JSON manifest",
      );
      return;
    }

    if (!publisher || !pluginName || !version) {
      setParseError("Publisher, plugin name, and version are required");
      return;
    }

    try {
      await publishMutation.mutateAsync({
        publisher,
        name: pluginName,
        version,
        manifest,
        release_notes: releaseNotes || undefined,
      });
      navigate({
        to: "/plugins/$publisher/$name",
        params: { publisher, name: pluginName },
      });
    } catch {
      // Error handled by mutation state
    }
  };

  return (
    <div className="container mx-auto py-8 max-w-2xl px-6">
      <div className="flex items-center space-x-4 mb-6">
        <Button
          variant="ghost"
          size="icon"
          onClick={() => navigate({ to: "/plugins/browse" })}
        >
          <Icons.ArrowLeft className="h-4 w-4" />
        </Button>
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Publish Plugin</h1>
          <p className="text-muted-foreground mt-1">
            Publish a new plugin version to the registry
          </p>
        </div>
      </div>

      <form onSubmit={handleSubmit} className="space-y-6">
        <Card>
          <CardHeader>
            <CardTitle>Plugin Details</CardTitle>
            <CardDescription>
              Basic information about your plugin
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="publisher">Publisher</Label>
              <Input
                id="publisher"
                placeholder="your-publisher-name"
                value={publisher}
                onChange={(e) => setPublisher(e.target.value)}
                required
              />
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="name">Plugin Name</Label>
                <Input
                  id="name"
                  placeholder="my-plugin"
                  value={pluginName}
                  onChange={(e) => setPluginName(e.target.value)}
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="version">Version</Label>
                <Input
                  id="version"
                  placeholder="1.0.0"
                  value={version}
                  onChange={(e) => setVersion(e.target.value)}
                  required
                />
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Manifest (JSON)</CardTitle>
            <CardDescription>
              Your plugin manifest as JSON
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Textarea
              className="font-mono text-sm min-h-[300px]"
              value={manifestJson}
              onChange={(e) => handleManifestChange(e.target.value)}
            />
            {parseError && (
              <p className="text-sm text-destructive mt-2">{parseError}</p>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Release Notes</CardTitle>
            <CardDescription>
              What's new in this version (optional)
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Textarea
              placeholder="Describe what changed in this version..."
              value={releaseNotes}
              onChange={(e) => setReleaseNotes(e.target.value)}
              className="min-h-[100px]"
            />
          </CardContent>
        </Card>

        {publishMutation.error && (
          <Card className="border-destructive">
            <CardContent className="pt-6">
              <p className="text-sm text-destructive">
                {publishMutation.error instanceof Error
                  ? publishMutation.error.message
                  : "Failed to publish plugin"}
              </p>
            </CardContent>
          </Card>
        )}

        <Button
          type="submit"
          className="w-full"
          disabled={publishMutation.isPending}
        >
          {publishMutation.isPending ? (
            <>
              <Icons.Loader className="h-4 w-4 mr-2 animate-spin" />
              Publishing...
            </>
          ) : (
            <>
              <Icons.Upload className="h-4 w-4 mr-2" />
              Publish Version
            </>
          )}
        </Button>
      </form>
    </div>
  );
}
