import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DangerZoneCard } from "../danger-zone-card";
import type { Project } from "~/lib/api";

// Mock AlertDialog to render inline (same pattern as Sheet mock)
vi.mock("~/components/ui/alert-dialog", () => ({
  AlertDialog: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="alert-dialog">{children}</div>
  ),
  AlertDialogTrigger: ({
    children,
    asChild,
  }: {
    children: React.ReactNode;
    asChild?: boolean;
  }) => <div data-testid="alert-dialog-trigger">{asChild ? children : children}</div>,
  AlertDialogContent: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="alert-dialog-content">{children}</div>
  ),
  AlertDialogHeader: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  AlertDialogTitle: ({ children }: { children: React.ReactNode }) => (
    <h2>{children}</h2>
  ),
  AlertDialogDescription: ({ children }: { children: React.ReactNode }) => (
    <p>{children}</p>
  ),
  AlertDialogFooter: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  AlertDialogCancel: ({
    children,
    onClick,
  }: {
    children: React.ReactNode;
    onClick?: () => void;
  }) => (
    <button type="button" onClick={onClick}>
      {children}
    </button>
  ),
  AlertDialogAction: ({
    children,
    onClick,
    disabled,
    variant: _variant,
  }: {
    children: React.ReactNode;
    onClick?: () => void;
    disabled?: boolean;
    variant?: string;
  }) => (
    <button
      type="button"
      data-testid="delete-action-button"
      onClick={onClick}
      disabled={disabled}
    >
      {children}
    </button>
  ),
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

describe("DangerZoneCard", () => {
  it("renders 'Danger Zone' title and 'Delete Project' button", () => {
    render(
      <DangerZoneCard
        project={makeProject()}
        onDelete={vi.fn()}
        deletePending={false}
      />,
    );
    expect(screen.getByText("Danger Zone")).toBeTruthy();
    // There are two "Delete Project" texts: the trigger button and the action button
    const deleteButtons = screen.getAllByText("Delete Project");
    expect(deleteButtons.length).toBeGreaterThanOrEqual(1);
  });

  it("delete action button disabled until correct slug is typed", () => {
    render(
      <DangerZoneCard
        project={makeProject({ slug: "my-project" })}
        onDelete={vi.fn()}
        deletePending={false}
      />,
    );
    const actionButton = screen.getByTestId("delete-action-button");
    expect(actionButton).toBeDisabled();
  });

  it("typing correct project slug enables the delete action button", async () => {
    const user = userEvent.setup();
    render(
      <DangerZoneCard
        project={makeProject({ slug: "my-project" })}
        onDelete={vi.fn()}
        deletePending={false}
      />,
    );
    const input = screen.getByPlaceholderText("my-project");
    await user.type(input, "my-project");

    const actionButton = screen.getByTestId("delete-action-button");
    expect(actionButton).not.toBeDisabled();
  });

  it("clicking delete action calls onDelete", async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();
    render(
      <DangerZoneCard
        project={makeProject({ slug: "my-project" })}
        onDelete={onDelete}
        deletePending={false}
      />,
    );
    const input = screen.getByPlaceholderText("my-project");
    await user.type(input, "my-project");

    const actionButton = screen.getByTestId("delete-action-button");
    await user.click(actionButton);

    expect(onDelete).toHaveBeenCalledOnce();
  });

  it("shows error when deleteError is set", () => {
    render(
      <DangerZoneCard
        project={makeProject()}
        onDelete={vi.fn()}
        deletePending={false}
        deleteError="Failed to delete project"
      />,
    );
    expect(screen.getByText("Failed to delete project")).toBeTruthy();
  });

  it("shows 'Deleting...' when deletePending is true", async () => {
    const user = userEvent.setup();
    const { rerender } = render(
      <DangerZoneCard
        project={makeProject({ slug: "test-project" })}
        onDelete={vi.fn()}
        deletePending={false}
      />,
    );
    // Type slug first so button is enabled (but pending will disable it)
    const input = screen.getByPlaceholderText("test-project");
    await user.type(input, "test-project");

    rerender(
      <DangerZoneCard
        project={makeProject({ slug: "test-project" })}
        onDelete={vi.fn()}
        deletePending={true}
      />,
    );
    expect(screen.getByText("Deleting...")).toBeTruthy();
    const actionButton = screen.getByTestId("delete-action-button");
    expect(actionButton).toBeDisabled();
  });
});
