import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardAction,
  CardContent,
  CardFooter,
} from "../card";

describe("Card", () => {
  it("renders with data-slot='card'", () => {
    render(<Card>Content</Card>);
    const card = screen.getByText("Content");
    expect(card.getAttribute("data-slot")).toBe("card");
  });

  it("forwards className", () => {
    render(<Card className="custom-class">Content</Card>);
    const card = screen.getByText("Content");
    expect(card.className).toContain("custom-class");
  });

  it("renders children", () => {
    render(
      <Card>
        <span>Child Element</span>
      </Card>,
    );
    expect(screen.getByText("Child Element")).toBeTruthy();
  });
});

describe("CardHeader", () => {
  it("renders with data-slot='card-header'", () => {
    render(<CardHeader>Header</CardHeader>);
    const header = screen.getByText("Header");
    expect(header.getAttribute("data-slot")).toBe("card-header");
  });

  it("forwards className", () => {
    render(<CardHeader className="header-cls">Header</CardHeader>);
    expect(screen.getByText("Header").className).toContain("header-cls");
  });
});

describe("CardTitle", () => {
  it("renders with data-slot='card-title'", () => {
    render(<CardTitle>Title</CardTitle>);
    expect(screen.getByText("Title").getAttribute("data-slot")).toBe(
      "card-title",
    );
  });

  it("forwards className", () => {
    render(<CardTitle className="title-cls">Title</CardTitle>);
    expect(screen.getByText("Title").className).toContain("title-cls");
  });
});

describe("CardDescription", () => {
  it("renders with data-slot='card-description'", () => {
    render(<CardDescription>Desc</CardDescription>);
    expect(screen.getByText("Desc").getAttribute("data-slot")).toBe(
      "card-description",
    );
  });
});

describe("CardAction", () => {
  it("renders with data-slot='card-action'", () => {
    render(<CardAction>Action</CardAction>);
    expect(screen.getByText("Action").getAttribute("data-slot")).toBe(
      "card-action",
    );
  });
});

describe("CardContent", () => {
  it("renders with data-slot='card-content'", () => {
    render(<CardContent>Main content</CardContent>);
    expect(screen.getByText("Main content").getAttribute("data-slot")).toBe(
      "card-content",
    );
  });

  it("forwards className", () => {
    render(<CardContent className="content-cls">Content</CardContent>);
    expect(screen.getByText("Content").className).toContain("content-cls");
  });
});

describe("CardFooter", () => {
  it("renders with data-slot='card-footer'", () => {
    render(<CardFooter>Footer</CardFooter>);
    expect(screen.getByText("Footer").getAttribute("data-slot")).toBe(
      "card-footer",
    );
  });
});

describe("Card composition", () => {
  it("renders full card structure", () => {
    render(
      <Card>
        <CardHeader>
          <CardTitle>My Card</CardTitle>
          <CardDescription>A description</CardDescription>
          <CardAction>Edit</CardAction>
        </CardHeader>
        <CardContent>Body content</CardContent>
        <CardFooter>Footer content</CardFooter>
      </Card>,
    );

    expect(screen.getByText("My Card")).toBeTruthy();
    expect(screen.getByText("A description")).toBeTruthy();
    expect(screen.getByText("Edit")).toBeTruthy();
    expect(screen.getByText("Body content")).toBeTruthy();
    expect(screen.getByText("Footer content")).toBeTruthy();
  });
});
