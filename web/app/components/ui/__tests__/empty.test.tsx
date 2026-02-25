import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  Empty,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
  EmptyDescription,
  EmptyContent,
} from "../empty";

describe("Empty", () => {
  it("renders with data-slot='empty'", () => {
    render(<Empty>Content</Empty>);
    const el = screen.getByText("Content");
    expect(el.getAttribute("data-slot")).toBe("empty");
  });

  it("forwards className", () => {
    render(<Empty className="custom">Content</Empty>);
    expect(screen.getByText("Content").className).toContain("custom");
  });

  it("has dashed border class", () => {
    render(<Empty>Content</Empty>);
    expect(screen.getByText("Content").className).toContain("border-dashed");
  });
});

describe("EmptyHeader", () => {
  it("renders with data-slot='empty-header'", () => {
    render(<EmptyHeader>Header</EmptyHeader>);
    expect(screen.getByText("Header").getAttribute("data-slot")).toBe(
      "empty-header",
    );
  });
});

describe("EmptyMedia", () => {
  it("renders with data-slot='empty-icon'", () => {
    render(<EmptyMedia>Icon</EmptyMedia>);
    expect(screen.getByText("Icon").getAttribute("data-slot")).toBe(
      "empty-icon",
    );
  });

  it("has default variant with bg-transparent", () => {
    render(<EmptyMedia>Icon</EmptyMedia>);
    const el = screen.getByText("Icon");
    expect(el.getAttribute("data-variant")).toBe("default");
    expect(el.className).toContain("bg-transparent");
  });

  it("supports icon variant", () => {
    render(<EmptyMedia variant="icon">Icon</EmptyMedia>);
    const el = screen.getByText("Icon");
    expect(el.getAttribute("data-variant")).toBe("icon");
    expect(el.className).toContain("bg-muted");
  });

  it("forwards className", () => {
    render(<EmptyMedia className="extra">Icon</EmptyMedia>);
    expect(screen.getByText("Icon").className).toContain("extra");
  });
});

describe("EmptyTitle", () => {
  it("renders with data-slot='empty-title'", () => {
    render(<EmptyTitle>No items</EmptyTitle>);
    expect(screen.getByText("No items").getAttribute("data-slot")).toBe(
      "empty-title",
    );
  });

  it("has font-medium class", () => {
    render(<EmptyTitle>Title</EmptyTitle>);
    expect(screen.getByText("Title").className).toContain("font-medium");
  });
});

describe("EmptyDescription", () => {
  it("renders with data-slot='empty-description'", () => {
    render(<EmptyDescription>Description text</EmptyDescription>);
    expect(
      screen.getByText("Description text").getAttribute("data-slot"),
    ).toBe("empty-description");
  });

  it("has muted foreground class", () => {
    render(<EmptyDescription>Desc</EmptyDescription>);
    expect(screen.getByText("Desc").className).toContain(
      "text-muted-foreground",
    );
  });
});

describe("EmptyContent", () => {
  it("renders with data-slot='empty-content'", () => {
    render(<EmptyContent>Body</EmptyContent>);
    expect(screen.getByText("Body").getAttribute("data-slot")).toBe(
      "empty-content",
    );
  });
});

describe("Empty composition", () => {
  it("renders full compound component structure", () => {
    render(
      <Empty>
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <svg data-testid="icon" />
          </EmptyMedia>
          <EmptyTitle>No results</EmptyTitle>
          <EmptyDescription>Try adjusting your search</EmptyDescription>
        </EmptyHeader>
        <EmptyContent>
          <button>Create new</button>
        </EmptyContent>
      </Empty>,
    );

    expect(screen.getByText("No results")).toBeTruthy();
    expect(screen.getByText("Try adjusting your search")).toBeTruthy();
    expect(screen.getByText("Create new")).toBeTruthy();
    expect(screen.getByTestId("icon")).toBeTruthy();
  });
});
