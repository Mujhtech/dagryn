import type { Project } from "~/lib/api";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Label } from "~/components/ui/label";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "~/components/ui/alert-dialog";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "~/components/ui/card";
import { Separator } from "~/components/ui/separator";
import { Icons } from "~/components/icons";

type DangerZoneCardProps = {
  project: Project;
  deleteConfirmText: string;
  setDeleteConfirmText: (value: string) => void;
  canDelete: boolean;
  deletePending: boolean;
  deleteError?: string;
  onDelete: () => void;
};

export function DangerZoneCard({
  project,
  deleteConfirmText,
  setDeleteConfirmText,
  canDelete,
  deletePending,
  deleteError,
  onDelete,
}: DangerZoneCardProps) {
  return (
    <Card className="border-destructive/50 py-6">
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-destructive">
          <Icons.Warning className="h-5 w-5" />
          Danger Zone
        </CardTitle>
        <CardDescription>Irreversible and destructive actions.</CardDescription>
      </CardHeader>
      <CardContent>
        <div className="flex items-start justify-between gap-4">
          <div className="space-y-1">
            <p className="font-medium">Delete this project</p>
            <p className="text-sm text-muted-foreground">
              Once you delete a project, there is no going back. All runs, logs, and associated data
              will be permanently removed.
            </p>
          </div>
          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button variant="destructive">
                <Icons.Trash className="mr-2 h-4 w-4" />
                Delete Project
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>Are you absolutely sure?</AlertDialogTitle>
                <AlertDialogDescription>
                  This action cannot be undone. This will permanently delete the <strong>{project.name}</strong>
                  {" "}
                  project and all of its data including runs, logs, and configurations.
                </AlertDialogDescription>
              </AlertDialogHeader>

              <Separator />

              <div className="space-y-2">
                <Label htmlFor="confirm-delete">
                  Type <code className="bg-muted px-1 rounded">{project.slug}</code> to confirm:
                </Label>
                <Input
                  id="confirm-delete"
                  value={deleteConfirmText}
                  onChange={(event) => setDeleteConfirmText(event.target.value)}
                  placeholder={project.slug}
                />
              </div>

              {deleteError ? (
                <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
                  {deleteError}
                </div>
              ) : null}

              <AlertDialogFooter>
                <AlertDialogCancel onClick={() => setDeleteConfirmText("")}>Cancel</AlertDialogCancel>
                <AlertDialogAction
                  variant="destructive"
                  onClick={onDelete}
                  disabled={!canDelete || deletePending}
                >
                  {deletePending ? (
                    <>
                      <Icons.Loader className="mr-2 h-4 w-4 animate-spin" />
                      Deleting...
                    </>
                  ) : (
                    "Delete Project"
                  )}
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </div>
      </CardContent>
    </Card>
  );
}
