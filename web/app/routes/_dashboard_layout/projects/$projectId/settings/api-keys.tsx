import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useProjectAPIKeys } from "~/hooks/queries";
import { useCreateProjectAPIKey, useRevokeProjectAPIKey } from "~/hooks/mutations";
import { APITokensCard } from "~/components/projects/settings/api-tokens-card";
import { generateMetadata } from "~/lib/metadata";

export const Route = createFileRoute(
  "/_dashboard_layout/projects/$projectId/settings/api-keys",
)({
  component: APIKeysSettingsPage,
  head: () => generateMetadata({ title: "API Keys" }),
});

function APIKeysSettingsPage() {
  const { projectId } = Route.useParams();

  const [createdKey, setCreatedKey] = useState<string | null>(null);

  const {
    data: apiKeys,
    isLoading: apiKeysLoading,
    error: apiKeysError,
  } = useProjectAPIKeys(projectId);

  const createAPIKeyMutation = useCreateProjectAPIKey(projectId);
  const revokeAPIKeyMutation = useRevokeProjectAPIKey(projectId);

  const handleCreateAPIKey = (values: { name: string; expires_in?: string }) => {
    createAPIKeyMutation.mutate(
      {
        name: values.name,
        expires_in: values.expires_in,
      },
      {
        onSuccess: (data) => {
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
