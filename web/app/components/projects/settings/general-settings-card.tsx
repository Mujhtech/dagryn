import { FolderCog } from "lucide-react";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Label } from "~/components/ui/label";
import { Textarea } from "~/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "~/components/ui/select";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "~/components/ui/card";
import { Icons } from "~/components/icons";

type GeneralSettingsCardProps = {
  name: string;
  setName: (value: string) => void;
  description: string;
  setDescription: (value: string) => void;
  visibility: "public" | "private";
  setVisibility: (value: "public" | "private") => void;
  onSave: () => void;
  isSaving: boolean;
  saveError?: string;
  saveSuccess: boolean;
};

export function GeneralSettingsCard({
  name,
  setName,
  description,
  setDescription,
  visibility,
  setVisibility,
  onSave,
  isSaving,
  saveError,
  saveSuccess,
}: GeneralSettingsCardProps) {
  return (
    <Card className="py-6">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <FolderCog className="h-5 w-5" />
          General
        </CardTitle>
        <CardDescription>Update your project&apos;s basic information.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="name">Project Name</Label>
          <Input
            id="name"
            value={name}
            onChange={(event) => setName(event.target.value)}
            placeholder="My Project"
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="description">Description</Label>
          <Textarea
            id="description"
            value={description}
            onChange={(event) => setDescription(event.target.value)}
            placeholder="A brief description of your project"
            className="resize-none"
            rows={3}
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="visibility">Visibility</Label>
          <Select
            value={visibility}
            onValueChange={(value) => setVisibility(value as "public" | "private")}
          >
            <SelectTrigger className="w-full">
              <SelectValue placeholder="Select visibility" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="private">
                Private - Only you and team members can see this project
              </SelectItem>
              <SelectItem value="public">
                Public - Anyone can see this project
              </SelectItem>
            </SelectContent>
          </Select>
          <p className="text-xs text-muted-foreground">
            {visibility === "public"
              ? "Public projects are visible to everyone."
              : "Private projects are only visible to you and your team."}
          </p>
        </div>

        {saveError ? (
          <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
            {saveError}
          </div>
        ) : null}

        {saveSuccess ? (
          <div className="rounded-md bg-green-500/10 p-3 text-sm text-green-600 dark:text-green-400">
            Project settings updated successfully!
          </div>
        ) : null}
      </CardContent>
      <CardFooter>
        <Button onClick={onSave} disabled={isSaving || !name.trim()}>
          {isSaving ? (
            <>
              <Icons.Loader className="mr-2 h-4 w-4 animate-spin" />
              Saving...
            </>
          ) : (
            <>
              <Icons.FloppyDisk className="mr-2 h-4 w-4" />
              Save Changes
            </>
          )}
        </Button>
      </CardFooter>
    </Card>
  );
}
