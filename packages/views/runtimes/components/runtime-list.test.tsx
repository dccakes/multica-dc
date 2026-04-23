import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { AgentRuntime } from "@multica/core/types";
import { RuntimeList } from "./runtime-list";

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

vi.mock("@multica/core/workspace/queries", () => ({
  memberListOptions: () => ({
    queryKey: ["members", "ws-1"],
    queryFn: async () => [{ user_id: "user-1", name: "Owner Name" }],
  }),
}));

vi.mock("../../common/actor-avatar", () => ({
  ActorAvatar: () => <span data-testid="actor-avatar" />,
}));

const cloudRuntime: AgentRuntime = {
  id: "rt-cloud-1",
  workspace_id: "ws-1",
  daemon_id: "cloudrunner:1",
  name: "Vercel Cloud Runtime",
  runtime_mode: "cloud",
  provider: "codex",
  launch_header: "",
  status: "offline",
  device_info: "Vercel Sandbox",
  metadata: {},
  owner_id: "user-1",
  last_seen_at: null,
  created_at: "2026-04-23T00:00:00Z",
  updated_at: "2026-04-23T00:00:00Z",
};

const localRuntime: AgentRuntime = {
  ...cloudRuntime,
  id: "rt-local-1",
  name: "MacBook Local Runtime",
  runtime_mode: "local",
  owner_id: "user-1",
};

function renderList(runtimes: AgentRuntime[]) {
  const queryClient = new QueryClient();
  return render(
    <QueryClientProvider client={queryClient}>
      <RuntimeList
        runtimes={runtimes}
        selectedId={runtimes[0]?.id ?? ""}
        onSelect={() => {}}
        filter="mine"
        onFilterChange={() => {}}
        ownerFilter={null}
        onOwnerFilterChange={() => {}}
      />
    </QueryClientProvider>,
  );
}

describe("RuntimeList runtime mode label", () => {
  it("shows remote label for cloud runtime", async () => {
    renderList([cloudRuntime]);
    expect(await screen.findByText("remote")).toBeInTheDocument();
  });

  it("shows local label for local runtime", async () => {
    renderList([localRuntime]);
    expect(await screen.findByText("local")).toBeInTheDocument();
  });
});
