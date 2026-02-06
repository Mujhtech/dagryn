import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Badge } from "../badge";

describe("Badge component", () => {
  it("should render with default props", () => {
    render(<Badge>Default</Badge>);
    const badge = screen.getByText("Default");
    expect(badge).toBeInTheDocument();
    expect(badge).toHaveAttribute("data-slot", "badge");
    expect(badge).toHaveAttribute("data-variant", "default");
  });

  it("should render different variants", () => {
    const { rerender } = render(<Badge variant="secondary">Secondary</Badge>);
    expect(screen.getByText("Secondary")).toHaveAttribute(
      "data-variant",
      "secondary"
    );

    rerender(<Badge variant="destructive">Destructive</Badge>);
    expect(screen.getByText("Destructive")).toHaveAttribute(
      "data-variant",
      "destructive"
    );

    rerender(<Badge variant="outline">Outline</Badge>);
    expect(screen.getByText("Outline")).toHaveAttribute(
      "data-variant",
      "outline"
    );

    rerender(<Badge variant="ghost">Ghost</Badge>);
    expect(screen.getByText("Ghost")).toHaveAttribute("data-variant", "ghost");

    rerender(<Badge variant="link">Link</Badge>);
    expect(screen.getByText("Link")).toHaveAttribute("data-variant", "link");
  });

  it("should apply custom className", () => {
    render(<Badge className="custom-class">Custom</Badge>);
    expect(screen.getByText("Custom")).toHaveClass("custom-class");
  });

  it("should render as span by default", () => {
    render(<Badge>Span Badge</Badge>);
    const badge = screen.getByText("Span Badge");
    expect(badge.tagName).toBe("SPAN");
  });

  it("should forward additional props", () => {
    render(<Badge data-testid="test-badge">Test</Badge>);
    expect(screen.getByTestId("test-badge")).toBeInTheDocument();
  });

  it("should render children correctly", () => {
    render(
      <Badge>
        <span>Icon</span>
        Status
      </Badge>
    );
    expect(screen.getByText("Icon")).toBeInTheDocument();
    expect(screen.getByText("Status")).toBeInTheDocument();
  });
});
