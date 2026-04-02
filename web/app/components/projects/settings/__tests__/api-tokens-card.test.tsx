import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { APITokensCard } from "../api-tokens-card";
import type { APIKey } from "~/lib/api";

// Mock Select to a plain <select> since Radix Select doesn't work in jsdom
vi.mock("~/components/ui/select", () => ({
  Select: ({
    value,
    onValueChange,
    disabled,
  }: {
    children?: React.ReactNode;
    value?: string;
    onValueChange?: (v: string) => void;
    disabled?: boolean;
  }) => (
    <select
      data-testid="expiry-select"
      value={value}
      disabled={disabled}
      onChange={(e) => onValueChange?.(e.target.value)}
    >
      <option value="30d">30 days</option>
      <option value="90d">90 days</option>
      <option value="1y">1 year</option>
      <option value="no">No expiration</option>
    </select>
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

function makeAPIKey(overrides: Partial<APIKey> = {}): APIKey {
  return {
    id: "key-1",
    name: "Deploy Token",
    prefix: "dgn_abc",
    scope: "project",
    project_id: "proj-1",
    created_at: "2025-01-01T00:00:00Z",
    ...overrides,
  };
}

describe("APITokensCard", () => {
  const defaultProps = {
    apiKeys: [] as APIKey[],
    apiKeysLoading: false,
    createdKey: null,
    onCopyKey: vi.fn(),
    onCreateToken: vi.fn(),
    createPending: false,
    revokePending: false,
    onRevoke: vi.fn(),
  };

  it("renders 'API Tokens' title, token name input, and Create Token button", () => {
    render(<APITokensCard {...defaultProps} />);
    expect(screen.getByText("API Tokens")).toBeTruthy();
    expect(screen.getByPlaceholderText("Production deploy token")).toBeTruthy();
    expect(screen.getByText("Create Token")).toBeTruthy();
  });

  it("submit with valid name calls onCreateToken with { name, expires_in: '90d' }", async () => {
    const user = userEvent.setup();
    const onCreateToken = vi.fn();
    render(
      <APITokensCard {...defaultProps} onCreateToken={onCreateToken} />,
    );

    const nameInput = screen.getByPlaceholderText("Production deploy token");
    await user.type(nameInput, "CI Token");
    await user.click(screen.getByText("Create Token"));

    await waitFor(() => {
      expect(onCreateToken).toHaveBeenCalledWith({
        name: "CI Token",
        expires_in: "90d",
      });
    });
  });

  it("submit with empty name shows validation error", async () => {
    const user = userEvent.setup();
    const onCreateToken = vi.fn();
    render(
      <APITokensCard {...defaultProps} onCreateToken={onCreateToken} />,
    );

    // Don't type anything, just submit
    await user.click(screen.getByText("Create Token"));

    await waitFor(() => {
      expect(screen.getByText("Token name is required")).toBeTruthy();
    });
    expect(onCreateToken).not.toHaveBeenCalled();
  });

  it("shows created key and copy button when createdKey is set", () => {
    render(
      <APITokensCard
        {...defaultProps}
        createdKey="dgn_live_abcdef123456"
      />,
    );
    expect(screen.getByText("dgn_live_abcdef123456")).toBeTruthy();
  });

  it("shows 'No tokens yet' when apiKeys is empty", () => {
    render(<APITokensCard {...defaultProps} apiKeys={[]} />);
    expect(
      screen.getByText("No tokens yet. Create a token above to get started."),
    ).toBeTruthy();
  });

  it("renders existing tokens with revoke buttons", () => {
    const keys = [
      makeAPIKey({ id: "key-1", name: "Deploy Token" }),
      makeAPIKey({ id: "key-2", name: "CI Token" }),
    ];
    render(<APITokensCard {...defaultProps} apiKeys={keys} />);

    expect(screen.getByText("Deploy Token")).toBeTruthy();
    expect(screen.getByText("CI Token")).toBeTruthy();
    const revokeButtons = screen.getAllByText("Revoke");
    expect(revokeButtons).toHaveLength(2);
  });

  it("clicking revoke calls onRevoke(key.id)", async () => {
    const user = userEvent.setup();
    const onRevoke = vi.fn();
    const keys = [makeAPIKey({ id: "key-42", name: "My Token" })];
    render(
      <APITokensCard {...defaultProps} apiKeys={keys} onRevoke={onRevoke} />,
    );

    await user.click(screen.getByText("Revoke"));
    expect(onRevoke).toHaveBeenCalledWith("key-42");
  });
});
