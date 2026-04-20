import { useState } from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useProject } from "~/hooks/queries";
import { useUpdateProject, useDeleteProject } from "~/hooks/mutations";
import { GeneralSettingsCard, type GeneralSettingsFormValues } from "~/components/projects/settings/general-settings-card";
import { DangerZoneCard } from "~/components/projects/settings/danger-zone-card";
import { generateMetadata } from "~/lib/metadata";

export const Route = createFileRoute(
  "/_dashboard_layout/projects/$projectId/settings/",
)({
  component: GeneralSettingsPage,
  head: () => generateMetadata({ title: "General Settings" }),
});

function GeneralSettingsPage() {
  const { projectId } = Route.useParams();
  const navigate = useNavigate();

  const { data: project } = useProject(projectId);

  const [saveSuccess, setSaveSuccess] = useState(false);

  const updateProjectMutation = useUpdateProject(projectId);
  const deleteProjectMutation = useDeleteProject(projectId);

  const handleSave = (values: GeneralSettingsFormValues) => {
    updateProjectMutation.mutate(
      {
        name: values.name.trim(),
        description: values.description?.trim() || undefined,
        visibility: values.visibility,
        team_id:
          values.team_id && values.team_id !== "none" ? values.team_id : null,
      },
      {
        onSuccess: () => {
          setSaveSuccess(true);
          setTimeout(() => setSaveSuccess(false), 3000);
        },
      },
    );
  };

  const handleDelete = () => {
    deleteProjectMutation.mutate(undefined, {
      onSuccess: () => {
        navigate({ to: "/projects" });
      },
    });
  };

  if (!project) return null;

  return (
    <div className="space-y-6">
      <GeneralSettingsCard
        project={project}
        onSave={handleSave}
        isSaving={updateProjectMutation.isPending}
        saveError={updateProjectMutation.error?.message}
        saveSuccess={saveSuccess}
      />

      <DangerZoneCard
        project={project}
        onDelete={handleDelete}
        deletePending={deleteProjectMutation.isPending}
        deleteError={deleteProjectMutation.error?.message}
      />
    </div>
  );
}
