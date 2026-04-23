import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CloudCredentialsPanel } from "./cloud-credentials-panel";

const mockList = vi.hoisted(() => vi.fn());
const mockCreate = vi.hoisted(() => vi.fn());
const mockToastError = vi.hoisted(() => vi.fn());

vi.mock("@multica/core/api", () => ({
  api: {
    listVercelCloudRuntimeCredentials: () => mockList(),
    createVercelCloudRuntimeCredential: (...args: unknown[]) => mockCreate(...args),
    testVercelCloudRuntimeCredential: vi.fn(),
    bootstrapVercelCloudRuntimeCredential: vi.fn(),
    deleteVercelCloudRuntimeCredential: vi.fn(),
    updateVercelCloudRuntimeCredential: vi.fn(),
  },
}));

vi.mock("sonner", () => ({
  toast: {
    error: (...args: unknown[]) => mockToastError(...args),
    success: vi.fn(),
  },
}));

describe("CloudCredentialsPanel permission feedback", () => {
  beforeEach(() => {
    mockList.mockReset();
    mockCreate.mockReset();
    mockToastError.mockReset();
    mockList.mockResolvedValue([]);
  });

  it("shows explicit admin permission message when create is forbidden", async () => {
    mockCreate.mockRejectedValue(new Error("insufficient permissions"));
    const user = userEvent.setup();

    render(<CloudCredentialsPanel />);

    await user.type(screen.getByPlaceholderText("Name"), "Vercel");
    await user.type(screen.getByPlaceholderText("Vercel Token"), "token");
    await user.type(screen.getByPlaceholderText("Project ID"), "project");
    await user.click(screen.getByRole("button", { name: "Add Credential" }));

    await waitFor(() => {
      expect(mockToastError).toHaveBeenCalledWith(
        "Workspace admin permissions required for this action",
      );
    });
  });
});
