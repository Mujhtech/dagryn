import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useProjectAPIKeys } from "~/hooks/queries";
import { useCreateProjectAPIKey, useRevokeProjectAPIKey } from "~/hooks/mutations";
import { APITokensCard } from "~/components/projects/settings/api-tokens-card";

export const Route = createFileRoute(
  "/_dashboard_layout/projects/$projectId/settings/api-keys",
)({
  component: APIKeysSettingsPage,
});

function APIKeysSettingsPage() {
  const { projectId } = Route.useParams();

  const [apiKeyName, setApiKeyName] = useState("");
  const [apiKeyExpiry, setApiKeyExpiry] = useState<string>("90d");
  const [createdKey, setCreatedKey] = useState<string | null>(null);

  const {
    data: apiKeys,
    isLoading: apiKeysLoading,
    error: apiKeysError,
  } = useProjectAPIKeys(projectId);

  const createAPIKeyMutation = useCreateProjectAPIKey(projectId);
  const revokeAPIKeyMutation = useRevokeProjectAPIKey(projectId);

  const handleCreateAPIKey = () => {
    if (!apiKeyName.trim()) return;

    createAPIKeyMutation.mutate(
      {
        name: apiKeyName.trim(),
        expires_in: apiKeyExpiry === "no" ? undefined : apiKeyExpiry,
      },
      {
        onSuccess: (data) => {
          setApiKeyName("");
          setCreatedKey(data.key);
        },
      },
    );
  };

  const handleCopyKey = async () => {
    if (!createdKey) return;
    try {
      await navigator.clipboard.writeText(createdKey);
    } catch {
      // ignore
    }
  };

  return (
    <div className="space-y-6">
      <APITokensCard
        apiKeys={apiKeys}
        apiKeysLoading={apiKeysLoading}
        apiKeysError={apiKeysError?.message}
        apiKeyName={apiKeyName}
        setApiKeyName={setApiKeyName}
        apiKeyExpiry={apiKeyExpiry}
        setApiKeyExpiry={setApiKeyExpiry}
        createdKey={createdKey}
        onCopyKey={handleCopyKey}
        onCreateToken={handleCreateAPIKey}
        createPending={createAPIKeyMutation.isPending}
        revokePending={revokeAPIKeyMutation.isPending}
        onRevoke={(id) => revokeAPIKeyMutation.mutate(id)}
      />
    </div>
  );
}
