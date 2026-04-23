import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { runtimeKeys } from "./queries";
import type {
  UpdateIssueRuntimePolicyRequest,
  UpdateWorkspaceRuntimePolicyRequest,
} from "../types";

export function useDeleteRuntime(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (runtimeId: string) => api.deleteRuntime(runtimeId),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: runtimeKeys.all(wsId) });
    },
  });
}

export function useUpdateWorkspaceRuntimePolicy(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: UpdateWorkspaceRuntimePolicyRequest) =>
      api.updateWorkspaceRuntimePolicy(wsId, data),
    onSuccess: (policy) => {
      qc.setQueryData(runtimeKeys.workspacePolicy(wsId), policy);
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: runtimeKeys.policy(wsId) });
    },
  });
}

export function useUpdateIssueRuntimePolicy(wsId: string, issueId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: UpdateIssueRuntimePolicyRequest) =>
      api.updateIssueRuntimePolicy(issueId, data),
    onSuccess: (policy) => {
      qc.setQueryData(runtimeKeys.issuePolicy(wsId, issueId), policy);
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: runtimeKeys.policy(wsId) });
    },
  });
}
