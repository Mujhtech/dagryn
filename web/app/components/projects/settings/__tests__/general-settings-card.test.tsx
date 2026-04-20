import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GeneralSettingsCard } from "../general-settings-card";
import type { Project } from "~/lib/api";

vi.mock("~/hooks/queries", () => ({
  useTeams: vi.fn(() => ({
    data: {
      data: [
        { id: "team-1", name: "Team One" },
        { id: "team-2", name: "Team Two" },
      ],
    },
  })),
}));

// Mock Select to a plain <select> since Radix Select doesn't work in jsdom
vi.mock("~/components/ui/select", () => ({
  Select: ({
    value,
    onValueChange,
  }: {
    children?: React.ReactNode;
    value?: string;
    onValueChange?: (v: string) => void;
  }) => (
    <div data-testid="select-root">
      <select
        data-testid="visibility-select"
        value={value}
        onChange={(e) => onValueChange?.(e.target.value)}
      >
        <option value="private">Private</option>
        <option value="public">Public</option>
      </select>
    </div>
  ),
  SelectTrigger: ({ children }: { children: React.ReactNode }) => (
    <span>{children}</span>
  ),
  SelectValue: ({ placeholder }: { placeholder?: string }) => (
    <span>{placeholder}</span>
  ),
  SelectContent: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  SelectItem: ({
    children,
    value,
  }: {
    children: React.ReactNode;
    value: string;
  }) => <option value={value}>{children}</option>,
}));

import React from "react";

function makeProject(overrides: Partial<Project> = {}): Project {
  return {
    id: "proj-1",
    team_id: "team-1",
    name: "Test Project",
    slug: "test-project",
    description: "A test project",
    visibility: "private",
    member_count: 3,
    created_at: "2025-01-01T00:00:00Z",
    updated_at: "2025-01-01T00:00:00Z",
    ...overrides,
  };
}

describe("GeneralSettingsCard", () => {
  it("renders card with title 'General' and Save Changes button", () => {
    render(
      <GeneralSettingsCard
        project={makeProject()}
        onSave={vi.fn()}
        isSaving={false}
        saveSuccess={false}
      />,
    );
    expect(screen.getByText("General")).toBeTruthy();
    expect(screen.getByText("Save Changes")).toBeTruthy();
  });

  it("form fields initialize from project prop", () => {
    render(
      <GeneralSettingsCard
        project={makeProject({
          name: "My App",
          description: "App description",
          visibility: "public",
        })}
        onSave={vi.fn()}
        isSaving={false}
        saveSuccess={false}
      />,
    );
    const nameInput = screen.getByPlaceholderText("My Project") as HTMLInputElement;
    expect(nameInput.value).toBe("My App");

    const descInput = screen.getByPlaceholderText(
      "A brief description of your project",
    ) as HTMLTextAreaElement;
    expect(descInput.value).toBe("App description");

    const selectEl = screen.getAllByTestId("visibility-select")[1] as HTMLSelectElement;
    expect(selectEl.value).toBe("public");
  });

  it("submit with valid data calls onSave with { name, description, visibility }", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    render(
      <GeneralSettingsCard
        project={makeProject({ name: "Original", description: "Desc", visibility: "private" })}
        onSave={onSave}
        isSaving={false}
        saveSuccess={false}
      />,
    );

    const nameInput = screen.getByPlaceholderText("My Project");
    await user.clear(nameInput);
    await user.type(nameInput, "Updated Name");

    await user.click(screen.getByText("Save Changes"));

    await waitFor(() => {
      expect(onSave).toHaveBeenCalledWith({
        name: "Updated Name",
        description: "Desc",
        visibility: "private",
        team_id: "team-1",
      });
    });
  });

  it("does not call onSave when name is empty (validation blocks)", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    render(
      <GeneralSettingsCard
        project={makeProject({ name: "Some Name" })}
        onSave={onSave}
        isSaving={false}
        saveSuccess={false}
      />,
    );

    const nameInput = screen.getByPlaceholderText("My Project");
    await user.clear(nameInput);

    await user.click(screen.getByText("Save Changes"));

    await waitFor(() => {
      expect(screen.getByText("Project name is required")).toBeTruthy();
    });
    expect(onSave).not.toHaveBeenCalled();
  });

  it("shows saveError when prop is set", () => {
    render(
      <GeneralSettingsCard
        project={makeProject()}
        onSave={vi.fn()}
        isSaving={false}
        saveError="Something went wrong"
        saveSuccess={false}
      />,
    );
    expect(screen.getByText("Something went wrong")).toBeTruthy();
  });

  it("shows success banner when saveSuccess is true", () => {
    render(
      <GeneralSettingsCard
        project={makeProject()}
        onSave={vi.fn()}
        isSaving={false}
        saveSuccess={true}
      />,
    );
    expect(screen.getByText("Project settings updated successfully!")).toBeTruthy();
  });

  it("Save button disabled and shows 'Saving...' when isSaving is true", () => {
    render(
      <GeneralSettingsCard
        project={makeProject()}
        onSave={vi.fn()}
        isSaving={true}
        saveSuccess={false}
      />,
    );
    expect(screen.getByText("Saving...")).toBeTruthy();
    const button = screen.getByRole("button", { name: /saving/i });
    expect(button).toBeDisabled();
  });
});
