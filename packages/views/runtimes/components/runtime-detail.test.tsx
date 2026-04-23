// @vitest-environment jsdom

import { describe, expect, it, vi, beforeEach } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { AgentRuntime } from "@multica/core/types";

const mockListMembers = vi.hoisted(() => vi.fn());
const mockGetPolicy = vi.hoisted(() => vi.fn());
const mockUpdatePolicy = vi.hoisted(() => vi.fn());
const mockDeleteRuntime = vi.hoisted(() => vi.fn());
const mockUseAuthStore = vi.hoisted(() => vi.fn());
const mockUseWorkspaceId = vi.hoisted(() => vi.fn());

vi.mock("@multica/core/api", () => ({
  api: {
    listMembers: (...args: unknown[]) => mockListMembers(...args),
    getWorkspaceRuntimePolicy: (...args: unknown[]) => mockGetPolicy(...args),
    updateWorkspaceRuntimePolicy: (...args: unknown[]) => mockUpdatePolicy(...args),
    deleteRuntime: (...args: unknown[]) => mockDeleteRuntime(...args),
  },
}));

vi.mock("@multica/core/auth", () => ({
  useAuthStore: (selector: (state: any) => unknown) => mockUseAuthStore(selector),
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => mockUseWorkspaceId(),
}));

vi.mock("@multica/core/runtimes/mutations", () => ({
  useDeleteRuntime: () => ({ mutate: mockDeleteRuntime }),
  useUpdateWorkspaceRuntimePolicy: () => ({
    mutateAsync: mockUpdatePolicy,
    isPending: false,
  }),
}));

vi.mock("./provider-logo", () => ({
  ProviderLogo: () => <div data-testid="provider-logo" />,
}));

vi.mock("../../common/actor-avatar", () => ({
  ActorAvatar: () => <div data-testid="actor-avatar" />,
}));

vi.mock("./shared", () => ({
  StatusBadge: ({ status }: { status: string }) => <div data-testid="status-badge">{status}</div>,
  InfoField: ({ label, value }: { label: string; value: string }) => (
    <div>
      <div>{label}</div>
      <div>{value}</div>
    </div>
  ),
}));

vi.mock("./ping-section", () => ({
  PingSection: () => <div data-testid="ping-section" />,
}));

vi.mock("./update-section", () => ({
  UpdateSection: () => <div data-testid="update-section" />,
}));

vi.mock("./usage-section", () => ({
  UsageSection: () => <div data-testid="usage-section" />,
}));

import { RuntimeDetail } from "./runtime-detail";

const runtime: AgentRuntime = {
  id: "runtime-1",
  workspace_id: "ws-1",
  daemon_id: "daemon-1",
  name: "Vercel Runtime",
  runtime_mode: "cloud",
  provider: "vercel",
  launch_header: "",
  status: "online",
  device_info: "node24",
  metadata: {},
  owner_id: "user-1",
  last_seen_at: "2026-04-23T12:00:00Z",
  created_at: "2026-04-23T11:00:00Z",
  updated_at: "2026-04-23T11:30:00Z",
};

function renderDetail() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <RuntimeDetail runtime={runtime} />
    </QueryClientProvider>,
  );
}

describe("RuntimeDetail", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseWorkspaceId.mockReturnValue("ws-1");
  });

  it("shows editable runtime policy controls for workspace admins", async () => {
    mockUseAuthStore.mockImplementation((selector: (state: any) => unknown) =>
      selector({ user: { id: "user-1" } }),
    );
    mockListMembers.mockResolvedValue([
      { user_id: "user-1", role: "admin", name: "Ada", email: "ada@multica.ai", avatar_url: null },
    ]);
    mockGetPolicy.mockResolvedValue({
      workspace_id: "ws-1",
      monthly_budget_cents: 125000,
      remote_concurrency_limit: 2,
      default_parent_issue_budget_cents: 25000,
    });
    mockUpdatePolicy.mockResolvedValue({
      workspace_id: "ws-1",
      monthly_budget_cents: 175000,
      remote_concurrency_limit: 3,
      default_parent_issue_budget_cents: 40000,
    });

    renderDetail();

    const monthlyBudget = await screen.findByLabelText("Monthly Budget (cents)");
    const concurrency = screen.getByLabelText("Remote Concurrency Limit");
    const parentBudget = screen.getByLabelText("Default Parent Issue Budget (cents)");

    expect(monthlyBudget).toHaveValue(125000);
    expect(concurrency).toHaveValue(2);
    expect(parentBudget).toHaveValue(25000);

    await userEvent.clear(monthlyBudget);
    await userEvent.type(monthlyBudget, "175000");
    await userEvent.clear(concurrency);
    await userEvent.type(concurrency, "3");
    await userEvent.clear(parentBudget);
    await userEvent.type(parentBudget, "40000");

    await userEvent.click(screen.getByRole("button", { name: "Save Policy" }));

    await waitFor(() => {
      expect(mockUpdatePolicy).toHaveBeenCalledWith({
        monthly_budget_cents: 175000,
        remote_concurrency_limit: 3,
        default_parent_issue_budget_cents: 40000,
      });
    });
  });

  it("hides the editor for non-admin members", async () => {
    mockUseAuthStore.mockImplementation((selector: (state: any) => unknown) =>
      selector({ user: { id: "user-1" } }),
    );
    mockListMembers.mockResolvedValue([
      { user_id: "user-1", role: "member", name: "Ada", email: "ada@multica.ai", avatar_url: null },
    ]);
    mockGetPolicy.mockResolvedValue({
      workspace_id: "ws-1",
      monthly_budget_cents: 125000,
      remote_concurrency_limit: 2,
      default_parent_issue_budget_cents: 25000,
    });

    renderDetail();

    await waitFor(() => {
      expect(mockGetPolicy).not.toHaveBeenCalled();
    });

    expect(screen.queryByLabelText("Monthly Budget (cents)")).toBeNull();
    expect(screen.getByText("Workspace runtime policy is managed by admins.")).toBeInTheDocument();
  });
});
