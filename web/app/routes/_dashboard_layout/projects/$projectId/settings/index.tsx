import { useState, useEffect } from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useProject } from "~/hooks/queries";
import { useUpdateProject, useDeleteProject } from "~/hooks/mutations";
import { GeneralSettingsCard } from "~/components/projects/settings/general-settings-card";
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

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [visibility, setVisibility] = useState<"public" | "private">("private");
  const [saveSuccess, setSaveSuccess] = useState(false);
  const [deleteConfirmText, setDeleteConfirmText] = useState("");

  const updateProjectMutation = useUpdateProject(projectId);
  const deleteProjectMutation = useDeleteProject(projectId);

  useEffect(() => {
    if (!project) return;

    setName(project.name || "");
    setDescription(project.description || "");
    setVisibility((project.visibility as "public" | "private") || "private");
  }, [project]);

  const handleSave = () => {
    if (!name.trim()) return;

    updateProjectMutation.mutate(
      {
        name: name.trim(),
        description: description.trim() || undefined,
        visibility,
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

  const canDelete = deleteConfirmText === project.slug;

  return (
    <div className="space-y-6">
      <GeneralSettingsCard
        name={name}
        setName={setName}
        description={description}
        setDescription={setDescription}
        visibility={visibility}
        setVisibility={setVisibility}
        onSave={handleSave}
        isSaving={updateProjectMutation.isPending}
        saveError={updateProjectMutation.error?.message}
        saveSuccess={saveSuccess}
      />

      <DangerZoneCard
        project={project}
        deleteConfirmText={deleteConfirmText}
        setDeleteConfirmText={setDeleteConfirmText}
        canDelete={canDelete}
        deletePending={deleteProjectMutation.isPending}
        deleteError={deleteProjectMutation.error?.message}
        onDelete={handleDelete}
      />
    </div>
  );
}
