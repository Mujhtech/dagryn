import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderWithProviders, screen, waitFor, userEvent } from "../../../../test/test-utils";
import { CreateProjectDialog } from "../create-project-dialog";

// --- Mocks ---

const mockMutate = vi.fn();
const mockReset = vi.fn();
let mockIsPending = false;
let mockError: Error | null = null;

vi.mock("~/hooks/mutations", () => ({
  useCreateProject: () => ({
    mutate: mockMutate,
    mutateAsync: vi.fn(),
    isPending: mockIsPending,
    error: mockError,
    reset: mockReset,
    isError: !!mockError,
    isIdle: !mockIsPending && !mockError,
    isSuccess: false,
    status: mockIsPending ? "pending" : mockError ? "error" : "idle",
    data: undefined,
    variables: undefined,
    failureCount: 0,
    failureReason: null,
    submittedAt: 0,
    context: undefined,
  }),
}));

let mockTeams: Array<{ id: string; name: string }> = [];

vi.mock("~/hooks/queries", () => ({
  useTeams: () => ({
    data: mockTeams.length > 0 ? { data: mockTeams } : undefined,
    isLoading: false,
    error: null,
  }),
}));

const mockNavigate = vi.fn();

vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => mockNavigate,
}));

// Mock Dialog - the trigger calls onOpenChange(true), content only renders when open
vi.mock("~/components/ui/dialog", () => {
  // We need a ref to store onOpenChange per Dialog instance
  let storedOnOpenChange: ((open: boolean) => void) | null = null;

  return {
    Dialog: ({
      children,
      open,
      onOpenChange,
    }: {
      children: React.ReactNode;
      open: boolean;
      onOpenChange: (open: boolean) => void;
    }) => {
      storedOnOpenChange = onOpenChange;
      return (
        <div data-testid="dialog" data-open={String(open)}>
          {children}
        </div>
      );
    },
    DialogContent: ({ children }: { children: React.ReactNode }) => {
      return <div data-testid="dialog-content">{children}</div>;
    },
    DialogDescription: ({ children }: { children: React.ReactNode }) => (
      <p>{children}</p>
    ),
    DialogFooter: ({ children }: { children: React.ReactNode }) => (
      <div data-testid="dialog-footer">{children}</div>
    ),
    DialogHeader: ({ children }: { children: React.ReactNode }) => (
      <div>{children}</div>
    ),
    DialogTitle: ({ children }: { children: React.ReactNode }) => (
      <h2>{children}</h2>
    ),
    DialogTrigger: ({
      children,
    }: {
      children: React.ReactNode;
      asChild?: boolean;
    }) => (
      <div
        data-testid="dialog-trigger"
        onClick={() => storedOnOpenChange?.(true)}
      >
        {children}
      </div>
    ),
  };
});

// Mock Select to render as native select element
vi.mock("~/components/ui/select", () => ({
  Select: ({
    children,
    value,
    onValueChange,
    disabled,
  }: {
    children: React.ReactNode;
    value?: string;
    onValueChange?: (v: string) => void;
    disabled?: boolean;
  }) => (
    <div data-testid="select-root">
      <select
        data-testid="team-select"
        value={value}
        disabled={disabled}
        onChange={(e) => onValueChange?.(e.target.value)}
      >
        {children}
      </select>
    </div>
  ),
  SelectContent: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
  SelectItem: ({
    children,
    value,
  }: {
    children: React.ReactNode;
    value: string;
  }) => <option value={value}>{children}</option>,
  SelectTrigger: () => null,
  SelectValue: () => null,
}));

beforeEach(() => {
  vi.clearAllMocks();
  mockIsPending = false;
  mockError = null;
  mockTeams = [];
});

describe("CreateProjectDialog", () => {
  it("renders 'New Project' trigger button", () => {
    renderWithProviders(<CreateProjectDialog />);
    expect(screen.getByText("New Project")).toBeTruthy();
  });

  it("dialog opens when trigger button clicked", async () => {
    const user = userEvent.setup();
    renderWithProviders(<CreateProjectDialog />);

    // Initially dialog is closed
    const dialog = screen.getByTestId("dialog");
    expect(dialog.getAttribute("data-open")).toBe("false");

    await user.click(screen.getByText("New Project"));

    // Dialog should now be open
    expect(dialog.getAttribute("data-open")).toBe("true");

    // Verify dialog content is present
    expect(
      screen.getByText(
        "Create a new workflow project. You can configure workflows after creation.",
      ),
    ).toBeTruthy();
  });

  it("typing name auto-generates slug", async () => {
    const user = userEvent.setup();
    renderWithProviders(<CreateProjectDialog />);

    await user.click(screen.getByText("New Project"));

    const nameInput = screen.getByPlaceholderText("My Project");
    await user.type(nameInput, "My Cool Project");

    const slugInput = screen.getByPlaceholderText("my-project");
    expect(slugInput).toHaveProperty("value", "my-cool-project");
  });

  it("manually editing slug stops auto-generation", async () => {
    const user = userEvent.setup();
    renderWithProviders(<CreateProjectDialog />);

    await user.click(screen.getByText("New Project"));

    const nameInput = screen.getByPlaceholderText("My Project");
    const slugInput = screen.getByPlaceholderText("my-project");

    // Type name first
    await user.type(nameInput, "First");
    expect(slugInput).toHaveProperty("value", "first");

    // Manually edit slug
    await user.clear(slugInput);
    await user.type(slugInput, "custom-slug");

    // Type more into name - slug should not change
    await user.type(nameInput, " Second");
    expect(slugInput).toHaveProperty("value", "custom-slug");
  });

  it("validation: submitting empty form shows name required error", async () => {
    const user = userEvent.setup();
    renderWithProviders(<CreateProjectDialog />);

    await user.click(screen.getByText("New Project"));

    // Use role to get the submit button (not the dialog title)
    const submitButton = screen.getByRole("button", { name: "Create Project" });
    await user.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText("Project name is required")).toBeTruthy();
    });

    expect(mockMutate).not.toHaveBeenCalled();
  });

  it("validation: invalid slug format shows error", async () => {
    const user = userEvent.setup();
    renderWithProviders(<CreateProjectDialog />);

    await user.click(screen.getByText("New Project"));

    const nameInput = screen.getByPlaceholderText("My Project");
    const slugInput = screen.getByPlaceholderText("my-project");

    await user.type(nameInput, "Test");
    await user.clear(slugInput);
    await user.type(slugInput, "INVALID SLUG!");

    const submitButton = screen.getByRole("button", { name: "Create Project" });
    await user.click(submitButton);

    await waitFor(() => {
      expect(
        screen.getByText("Slug must be lowercase alphanumeric with hyphens"),
      ).toBeTruthy();
    });

    expect(mockMutate).not.toHaveBeenCalled();
  });

  it("successful submission calls mutate with correct payload", async () => {
    const user = userEvent.setup();
    renderWithProviders(<CreateProjectDialog />);

    await user.click(screen.getByText("New Project"));

    const nameInput = screen.getByPlaceholderText("My Project");
    const descInput = screen.getByPlaceholderText(
      "A brief description of your project",
    );

    await user.type(nameInput, "Test Project");
    await user.type(descInput, "A test description");

    const submitButton = screen.getByRole("button", { name: "Create Project" });
    await user.click(submitButton);

    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalledTimes(1);
    });

    const [payload] = mockMutate.mock.calls[0];
    expect(payload).toEqual({
      name: "Test Project",
      slug: "test-project",
      description: "A test description",
      visibility: "private",
      team_id: undefined,
    });
  });

  it("team selector shown when teams exist", async () => {
    mockTeams = [
      { id: "team-1", name: "Team Alpha" },
      { id: "team-2", name: "Team Beta" },
    ];

    const user = userEvent.setup();
    renderWithProviders(<CreateProjectDialog />);

    await user.click(screen.getByText("New Project"));

    expect(screen.getByText("Team (optional)")).toBeTruthy();
    expect(screen.getByText("Team Alpha")).toBeTruthy();
    expect(screen.getByText("Team Beta")).toBeTruthy();
  });

  it("team selector hidden when no teams", async () => {
    mockTeams = [];

    const user = userEvent.setup();
    renderWithProviders(<CreateProjectDialog />);

    await user.click(screen.getByText("New Project"));

    expect(screen.queryByText("Team (optional)")).toBeNull();
  });

  it('"none" team_id sent as undefined', async () => {
    mockTeams = [{ id: "team-1", name: "Team Alpha" }];

    const user = userEvent.setup();
    renderWithProviders(<CreateProjectDialog />);

    await user.click(screen.getByText("New Project"));

    // Default is "none" - just fill in required fields
    const nameInput = screen.getByPlaceholderText("My Project");
    await user.type(nameInput, "Test");

    const submitButton = screen.getByRole("button", { name: "Create Project" });
    await user.click(submitButton);

    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalledTimes(1);
    });

    const [payload] = mockMutate.mock.calls[0];
    expect(payload.team_id).toBeUndefined();
  });

  it("form resets on dialog close", async () => {
    const user = userEvent.setup();
    renderWithProviders(<CreateProjectDialog />);

    // Open dialog
    await user.click(screen.getByText("New Project"));

    // Type something
    const nameInput = screen.getByPlaceholderText("My Project");
    await user.type(nameInput, "Something");
    expect(nameInput).toHaveProperty("value", "Something");

    // Close dialog by clicking Cancel
    const cancelButton = screen.getByRole("button", { name: "Cancel" });
    await user.click(cancelButton);

    // Verify reset was called on the mutation
    expect(mockReset).toHaveBeenCalled();

    // Re-open dialog
    await user.click(screen.getByText("New Project"));

    // Name should be empty again
    const nameInputAfter = screen.getByPlaceholderText("My Project");
    expect(nameInputAfter).toHaveProperty("value", "");
  });

  it("shows loading state during submission", () => {
    mockIsPending = true;
    renderWithProviders(<CreateProjectDialog />);

    // The dialog mock always renders content, and isPending is true,
    // so the submit button shows "Creating..."
    expect(screen.getByText("Creating...")).toBeTruthy();
  });

  it("shows error when mutation fails", async () => {
    mockError = new Error("Project slug already exists");

    const user = userEvent.setup();
    renderWithProviders(<CreateProjectDialog />);

    await user.click(screen.getByText("New Project"));

    expect(screen.getByText("Project slug already exists")).toBeTruthy();
  });
});
