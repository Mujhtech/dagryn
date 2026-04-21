import { createFileRoute } from "@tanstack/react-router";
import { toast } from "sonner";
import { EnvVarsCard } from "~/components/projects/settings/env-vars-card";
import { useProjectEnvVars } from "~/hooks/queries";
import {
  useDeleteProjectEnvVar,
  useRotateProjectEnvVar,
  useSeedProjectEnvVars,
  useSetProjectEnvVar,
  useUpdateProjectEnvVar,
} from "~/hooks/mutations";
import type { SetProjectEnvVarInput } from "~/lib/api";
import { generateMetadata } from "~/lib/metadata";

export const Route = createFileRoute(
  "/_dashboard_layout/projects/$projectId/settings/env",
)({
  component: EnvSettingsPage,
  head: () => generateMetadata({ title: "Env & Secrets" }),
});

function EnvSettingsPage() {
  const { projectId } = Route.useParams();

  const { data: envVars, isLoading, error } = useProjectEnvVars(projectId);
  const setMutation = useSetProjectEnvVar(projectId);
  const seedMutation = useSeedProjectEnvVars(projectId);
  const rotateMutation = useRotateProjectEnvVar(projectId);
  const deleteMutation = useDeleteProjectEnvVar(projectId);
  const updateMutation = useUpdateProjectEnvVar(projectId);

  const handleSeed = (
    content: string,
    options: {
      environment?: string;
      branch?: string;
      secret: boolean;
      required: boolean;
    },
  ) => {
    const items = parseDotEnv(content, options);
    if (items.length === 0) {
      toast.error("No valid KEY=VALUE pairs found in seed input");
      return;
    }

    seedMutation.mutate(items, {
      onSuccess: (created) => {
        toast.success(`Seeded ${created.length} environment variables`);
      },
      onError: (err: unknown) => {
        toast.error(err instanceof Error ? err.message : "Failed to seed environment variables");
      },
    });
  };

  return (
    <div className="space-y-6">
      <EnvVarsCard
        envVars={envVars}
        loading={isLoading}
        error={error?.message}
        createPending={setMutation.isPending}
        deletePending={deleteMutation.isPending}
        rotatePending={rotateMutation.isPending}
        updatePending={updateMutation.isPending}
        seedPending={seedMutation.isPending}
        onCreate={(payload) =>
          setMutation.mutate(payload, {
            onSuccess: () => {
              toast.success(`Saved ${payload.key}`);
            },
            onError: (err: unknown) => {
              toast.error(err instanceof Error ? err.message : "Failed to save variable");
            },
          })
        }
        onDelete={(id) =>
          deleteMutation.mutate(id, {
            onSuccess: () => {
              toast.success("Deleted variable");
            },
            onError: (err: unknown) => {
              toast.error(err instanceof Error ? err.message : "Failed to delete variable");
            },
          })
        }
        onRotate={(payload) =>
          rotateMutation.mutate(payload, {
            onSuccess: () => {
              toast.success("Rotated secret");
            },
            onError: (err: unknown) => {
              toast.error(err instanceof Error ? err.message : "Failed to rotate secret");
            },
          })
        }
        onUpdate={(payload) =>
          updateMutation.mutate(payload, {
            onSuccess: () => {
              toast.success("Updated variable metadata");
            },
            onError: (err: unknown) => {
              toast.error(err instanceof Error ? err.message : "Failed to update variable");
            },
          })
        }
        onSeed={handleSeed}
      />
    </div>
  );
}

function parseDotEnv(
  content: string,
  options: {
    environment?: string;
    branch?: string;
    secret: boolean;
    required: boolean;
  },
): SetProjectEnvVarInput[] {
  const out: SetProjectEnvVarInput[] = [];
  const lines = content.split("\n");

  for (const rawLine of lines) {
    const line = rawLine.trim();
    if (!line || line.startsWith("#")) continue;
    const idx = line.indexOf("=");
    if (idx <= 0) continue;

    const key = line.slice(0, idx).trim();
    const rawValue = line.slice(idx + 1).trim();
    const value = rawValue.replace(/^"|"$/g, "");
    if (!key) continue;

    const item: SetProjectEnvVarInput = {
      key,
      value,
      environment: options.environment,
      branch: options.branch,
      required: options.required,
      secret: options.secret,
    };

    out.push(item);
  }

  return out;
}
