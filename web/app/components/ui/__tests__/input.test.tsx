import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Input } from "../input";

describe("Input", () => {
  it("renders with data-slot='input'", () => {
    render(<Input placeholder="Type here" />);
    const input = screen.getByPlaceholderText("Type here");
    expect(input.getAttribute("data-slot")).toBe("input");
  });

  it("renders with correct type", () => {
    render(<Input type="email" placeholder="Email" />);
    const input = screen.getByPlaceholderText("Email");
    expect(input.getAttribute("type")).toBe("email");
  });

  it("renders with password type", () => {
    render(<Input type="password" placeholder="Password" />);
    const input = screen.getByPlaceholderText("Password");
    expect(input.getAttribute("type")).toBe("password");
  });

  it("renders in disabled state", () => {
    render(<Input disabled placeholder="Disabled" />);
    const input = screen.getByPlaceholderText("Disabled");
    expect(input).toBeDisabled();
  });

  it("forwards custom className", () => {
    render(<Input className="custom-input" placeholder="Test" />);
    const input = screen.getByPlaceholderText("Test");
    expect(input.className).toContain("custom-input");
  });

  it("has aria-invalid styling support", () => {
    render(<Input aria-invalid="true" placeholder="Invalid" />);
    const input = screen.getByPlaceholderText("Invalid");
    expect(input.getAttribute("aria-invalid")).toBe("true");
  });

  it("forwards additional HTML attributes", () => {
    render(
      <Input
        placeholder="Name"
        name="username"
        autoComplete="off"
        maxLength={50}
      />,
    );
    const input = screen.getByPlaceholderText("Name");
    expect(input.getAttribute("name")).toBe("username");
    expect(input.getAttribute("autocomplete")).toBe("off");
    expect(input.getAttribute("maxlength")).toBe("50");
  });

  it("renders as input element", () => {
    render(<Input placeholder="Test" />);
    const input = screen.getByPlaceholderText("Test");
    expect(input.tagName).toBe("INPUT");
  });
});
