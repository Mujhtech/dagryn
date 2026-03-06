import type { APIKey } from "~/lib/api";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Label } from "~/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "~/components/ui/select";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "~/components/ui/card";
import { Separator } from "~/components/ui/separator";
import { Badge } from "~/components/ui/badge";
import { Icons } from "~/components/icons";

type APITokensCardProps = {
  apiKeys?: APIKey[];
  apiKeysLoading: boolean;
  apiKeysError?: string;
  apiKeyName: string;
  setApiKeyName: (value: string) => void;
  apiKeyExpiry: string;
  setApiKeyExpiry: (value: string) => void;
  createdKey: string | null;
  onCopyKey: () => void;
  onCreateToken: () => void;
  createPending: boolean;
  revokePending: boolean;
  onRevoke: (id: string) => void;
};

export function APITokensCard({
  apiKeys,
  apiKeysLoading,
  apiKeysError,
  apiKeyName,
  setApiKeyName,
  apiKeyExpiry,
  setApiKeyExpiry,
  createdKey,
  onCopyKey,
  onCreateToken,
  createPending,
  revokePending,
  onRevoke,
}: APITokensCardProps) {
  return (
    <Card className="py-6">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Icons.Key className="h-5 w-5" />
          API Tokens
        </CardTitle>
        <CardDescription>
          Create and manage API tokens for CI/CD and other integrations. Tokens are scoped to
          this project.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {apiKeysError ? (
          <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
            {apiKeysError}
          </div>
        ) : null}

        <div className="space-y-2">
          <Label htmlFor="api-key-name">Token name</Label>
          <Input
            id="api-key-name"
            value={apiKeyName}
            onChange={(event) => setApiKeyName(event.target.value)}
            placeholder="Production deploy token"
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="api-key-expiry">Expiration</Label>
          <Select value={apiKeyExpiry} onValueChange={setApiKeyExpiry}>
            <SelectTrigger id="api-key-expiry" className="w-full">
              <SelectValue placeholder="Select expiration" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="30d">30 days</SelectItem>
              <SelectItem value="90d">90 days</SelectItem>
              <SelectItem value="1y">1 year</SelectItem>
              <SelectItem value="no">No expiration</SelectItem>
            </SelectContent>
          </Select>
          <p className="text-xs text-muted-foreground">
            You can revoke a token at any time. For most CI systems, <span className="font-medium">90 days</span> is a good default.
          </p>
        </div>

        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <Button onClick={onCreateToken} disabled={createPending || !apiKeyName.trim()}>
            {createPending ? (
              <>
                <Icons.Loader className="mr-2 h-4 w-4 animate-spin" />
                Creating...
              </>
            ) : (
              "Create Token"
            )}
          </Button>

          {createdKey ? (
            <div className="flex flex-1 items-center gap-2 rounded-md bg-muted px-3 py-2">
              <Icons.Key className="h-4 w-4 text-muted-foreground" />
              <span className="font-mono text-xs truncate">{createdKey}</span>
              <Button
                type="button"
                variant="outline"
                size="icon"
                className="h-7 w-7"
                onClick={onCopyKey}
              >
                <Icons.Copy className="h-3 w-3" />
              </Button>
            </div>
          ) : null}
        </div>

        <Separator />

        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <p className="text-sm font-medium">Existing tokens</p>
            {apiKeysLoading ? (
              <Icons.Loader className="h-4 w-4 animate-spin text-muted-foreground" />
            ) : null}
          </div>

          {!apiKeysLoading && (!apiKeys || apiKeys.length === 0) ? (
            <p className="text-sm text-muted-foreground">
              No tokens yet. Create a token above to get started.
            </p>
          ) : null}

          {apiKeys && apiKeys.length > 0 ? (
            <div className="space-y-2">
              {apiKeys.map((key) => (
                <div
                  key={key.id}
                  className="flex items-start justify-between gap-3 rounded-md border bg-card px-3 py-2"
                >
                  <div className="space-y-1">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-medium">{key.name}</span>
                      <Badge variant="outline" className="text-[10px]">
                        {key.scope}
                      </Badge>
                      {key.expires_at ? (
                        <Badge
                          variant="outline"
                          className="text-[10px] text-amber-700 dark:text-amber-400"
                        >
                          Expires {new Date(key.expires_at).toLocaleDateString()}
                        </Badge>
                      ) : null}
                    </div>
                    <p className="text-xs font-mono text-muted-foreground">{key.prefix}••••••</p>
                    <p className="text-xs text-muted-foreground">
                      Created {new Date(key.created_at).toLocaleDateString()}
                      {key.last_used_at
                        ? ` · Last used ${new Date(key.last_used_at).toLocaleDateString()}`
                        : " · Never used"}
                    </p>
                  </div>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    className="shrink-0"
                    disabled={revokePending}
                    onClick={() => onRevoke(key.id)}
                  >
                    Revoke
                  </Button>
                </div>
              ))}
            </div>
          ) : null}
        </div>
      </CardContent>
    </Card>
  );
}
