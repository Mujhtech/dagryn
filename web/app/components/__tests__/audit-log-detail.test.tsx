import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { AuditLogDetailSheet } from "../audit-log-detail";
import type { AuditLogEntry } from "~/lib/api";

// Mock Sheet components to simplify rendering
vi.mock("~/components/ui/sheet", () => ({
  Sheet: ({
    children,
    open,
  }: {
    children: React.ReactNode;
    open: boolean;
  }) => (open ? <div data-testid="sheet">{children}</div> : null),
  SheetContent: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="sheet-content">{children}</div>
  ),
  SheetHeader: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  SheetTitle: ({ children }: { children: React.ReactNode }) => (
    <h2>{children}</h2>
  ),
  SheetDescription: ({ children }: { children: React.ReactNode }) => (
    <p>{children}</p>
  ),
}));

function makeEntry(overrides: Partial<AuditLogEntry> = {}): AuditLogEntry {
  return {
    id: "entry-1",
    team_id: "team-1",
    actor_email: "user@example.com",
    actor_type: "user",
    action: "project.created",
    category: "project",
    resource_type: "project",
    description: "Created project X",
    entry_hash: "abc123hash",
    sequence_num: 42,
    created_at: "2025-01-15T10:30:00Z",
    ...overrides,
  };
}

describe("AuditLogDetailSheet", () => {
  it("renders nothing when entry is null", () => {
    const { container } = render(
      <AuditLogDetailSheet entry={null} open={true} onOpenChange={vi.fn()} />,
    );
    expect(container.innerHTML).toBe("");
  });

  it("renders nothing when open is false", () => {
    const entry = makeEntry();
    render(
      <AuditLogDetailSheet entry={entry} open={false} onOpenChange={vi.fn()} />,
    );
    // Sheet mock returns null when open is false
    expect(screen.queryByTestId("sheet")).toBeNull();
  });

  it("renders entry ID", () => {
    const entry = makeEntry({ id: "entry-42" });
    render(
      <AuditLogDetailSheet entry={entry} open={true} onOpenChange={vi.fn()} />,
    );
    expect(screen.getByText("entry-42")).toBeTruthy();
  });

  it("renders action badge", () => {
    const entry = makeEntry({ action: "team.member_added" });
    render(
      <AuditLogDetailSheet entry={entry} open={true} onOpenChange={vi.fn()} />,
    );
    expect(screen.getByText("team.member_added")).toBeTruthy();
  });

  it("renders actor email for user type", () => {
    const entry = makeEntry({
      actor_type: "user",
      actor_email: "alice@example.com",
    });
    render(
      <AuditLogDetailSheet entry={entry} open={true} onOpenChange={vi.fn()} />,
    );
    expect(screen.getByText("alice@example.com")).toBeTruthy();
  });

  it("renders 'System' for system actor type", () => {
    const entry = makeEntry({
      actor_type: "system",
      actor_email: "",
    });
    render(
      <AuditLogDetailSheet entry={entry} open={true} onOpenChange={vi.fn()} />,
    );
    expect(screen.getByText("System")).toBeTruthy();
  });

  it("renders timestamp", () => {
    const entry = makeEntry({ created_at: "2025-06-01T12:00:00Z" });
    render(
      <AuditLogDetailSheet entry={entry} open={true} onOpenChange={vi.fn()} />,
    );
    // The exact format depends on locale, so just check the Timestamp label exists
    expect(screen.getByText("Timestamp")).toBeTruthy();
  });

  it("renders category and description", () => {
    const entry = makeEntry({
      category: "security",
      description: "Password changed",
    });
    render(
      <AuditLogDetailSheet entry={entry} open={true} onOpenChange={vi.fn()} />,
    );
    expect(screen.getByText("security")).toBeTruthy();
    expect(screen.getByText("Password changed")).toBeTruthy();
  });

  it("renders actor_id when present", () => {
    const entry = makeEntry({ actor_id: "uid-abc" });
    render(
      <AuditLogDetailSheet entry={entry} open={true} onOpenChange={vi.fn()} />,
    );
    expect(screen.getByText("uid-abc")).toBeTruthy();
    expect(screen.getByText("Actor ID")).toBeTruthy();
  });

  it("does not render Actor ID row when actor_id is absent", () => {
    const entry = makeEntry({ actor_id: undefined });
    render(
      <AuditLogDetailSheet entry={entry} open={true} onOpenChange={vi.fn()} />,
    );
    expect(screen.queryByText("Actor ID")).toBeNull();
  });

  it("renders resource_id when present", () => {
    const entry = makeEntry({ resource_id: "res-123" });
    render(
      <AuditLogDetailSheet entry={entry} open={true} onOpenChange={vi.fn()} />,
    );
    expect(screen.getByText("res-123")).toBeTruthy();
  });

  it("does not render Resource ID when absent", () => {
    const entry = makeEntry({ resource_id: undefined });
    render(
      <AuditLogDetailSheet entry={entry} open={true} onOpenChange={vi.fn()} />,
    );
    expect(screen.queryByText("Resource ID")).toBeNull();
  });

  it("renders metadata JSON when present", () => {
    const entry = makeEntry({
      metadata: { key: "value", nested: { a: 1 } },
    });
    render(
      <AuditLogDetailSheet entry={entry} open={true} onOpenChange={vi.fn()} />,
    );
    expect(screen.getByText("Metadata")).toBeTruthy();
    // Check JSON is rendered
    expect(screen.getByText(/\"key\": \"value\"/)).toBeTruthy();
  });

  it("does not render metadata section when metadata is empty", () => {
    const entry = makeEntry({ metadata: {} });
    render(
      <AuditLogDetailSheet entry={entry} open={true} onOpenChange={vi.fn()} />,
    );
    expect(screen.queryByText("Metadata")).toBeNull();
  });

  it("does not render metadata section when metadata is undefined", () => {
    const entry = makeEntry({ metadata: undefined });
    render(
      <AuditLogDetailSheet entry={entry} open={true} onOpenChange={vi.fn()} />,
    );
    expect(screen.queryByText("Metadata")).toBeNull();
  });

  it("renders entry hash", () => {
    const entry = makeEntry({ entry_hash: "sha256:deadbeef" });
    render(
      <AuditLogDetailSheet entry={entry} open={true} onOpenChange={vi.fn()} />,
    );
    expect(screen.getByText("sha256:deadbeef")).toBeTruthy();
  });

  it("renders prev_hash when present", () => {
    const entry = makeEntry({ prev_hash: "sha256:prev" });
    render(
      <AuditLogDetailSheet entry={entry} open={true} onOpenChange={vi.fn()} />,
    );
    expect(screen.getByText("sha256:prev")).toBeTruthy();
    expect(screen.getByText("Previous Hash")).toBeTruthy();
  });

  it("does not render Previous Hash when absent", () => {
    const entry = makeEntry({ prev_hash: undefined });
    render(
      <AuditLogDetailSheet entry={entry} open={true} onOpenChange={vi.fn()} />,
    );
    expect(screen.queryByText("Previous Hash")).toBeNull();
  });

  it("renders sequence number", () => {
    const entry = makeEntry({ sequence_num: 99 });
    render(
      <AuditLogDetailSheet entry={entry} open={true} onOpenChange={vi.fn()} />,
    );
    expect(screen.getByText("#99")).toBeTruthy();
  });
});
