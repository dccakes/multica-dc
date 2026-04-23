"use client";

import { useEffect, useState } from "react";
import { Loader2, Trash2, Save } from "lucide-react";
import { toast } from "sonner";
import { useQuery } from "@tanstack/react-query";
import type { AgentRuntime } from "@multica/core/types";
import { useAuthStore } from "@multica/core/auth";
import { useWorkspaceId } from "@multica/core/hooks";
import { memberListOptions } from "@multica/core/workspace/queries";
import { useDeleteRuntime } from "@multica/core/runtimes/mutations";
import {
  workspaceRuntimePolicyOptions,
} from "@multica/core/runtimes/queries";
import { useUpdateWorkspaceRuntimePolicy } from "@multica/core/runtimes/mutations";
import { Button } from "@multica/ui/components/ui/button";
import { Card, CardContent } from "@multica/ui/components/ui/card";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@multica/ui/components/ui/alert-dialog";
import { ActorAvatar } from "../../common/actor-avatar";
import { formatLastSeen } from "../utils";
import { StatusBadge, InfoField } from "./shared";
import { ProviderLogo } from "./provider-logo";
import { PingSection } from "./ping-section";
import { UpdateSection } from "./update-section";
import { UsageSection } from "./usage-section";

function getCliVersion(metadata: Record<string, unknown>): string | null {
  if (
    metadata &&
    typeof metadata.cli_version === "string" &&
    metadata.cli_version
  ) {
    return metadata.cli_version;
  }
  return null;
}

function getLaunchedBy(metadata: Record<string, unknown>): string | null {
  if (
    metadata &&
    typeof metadata.launched_by === "string" &&
    metadata.launched_by
  ) {
    return metadata.launched_by;
  }
  return null;
}

function getExecutionStateLabel(metadata: Record<string, unknown>): string | null {
  const raw = metadata?.execution_state;
  if (typeof raw !== "string") return null;

  if (raw === "needs_human_intervention") return "needs intervention";
  if (raw === "paused_budget_blocked") return "paused (budget)";
  if (raw === "blocked_budget") return "blocked (budget)";
  return null;
}

function formatPolicyValue(value: number | null | undefined): string {
  if (value === null || value === undefined) return "—";
  return String(value);
}

export function RuntimeDetail({ runtime }: { runtime: AgentRuntime }) {
  const cliVersion =
    runtime.runtime_mode === "local" ? getCliVersion(runtime.metadata) : null;
  const launchedBy =
    runtime.runtime_mode === "local" ? getLaunchedBy(runtime.metadata) : null;
  const executionState = getExecutionStateLabel(runtime.metadata);

  const user = useAuthStore((s) => s.user);
  const wsId = useWorkspaceId();
  const { data: members = [] } = useQuery(memberListOptions(wsId));
  const deleteMutation = useDeleteRuntime(wsId);

  const [deleteOpen, setDeleteOpen] = useState(false);
  const [monthlyBudgetCents, setMonthlyBudgetCents] = useState("");
  const [remoteConcurrencyLimit, setRemoteConcurrencyLimit] = useState("");
  const [defaultParentIssueBudgetCents, setDefaultParentIssueBudgetCents] = useState("");

  // Resolve owner info
  const ownerMember = runtime.owner_id
    ? members.find((m) => m.user_id === runtime.owner_id) ?? null
    : null;

  // Permission check for delete
  const currentMember = user
    ? members.find((m) => m.user_id === user.id)
    : null;
  const isAdmin = currentMember
    ? currentMember.role === "owner" || currentMember.role === "admin"
    : false;
  const isRuntimeOwner = user && runtime.owner_id === user.id;
  const canDelete = isAdmin || isRuntimeOwner;
  const { data: workspacePolicy } = useQuery({
    ...workspaceRuntimePolicyOptions(wsId),
    enabled: Boolean(wsId && isAdmin),
  });
  const policyMutation = useUpdateWorkspaceRuntimePolicy(wsId);

  useEffect(() => {
    if (!workspacePolicy) return;
    setMonthlyBudgetCents(formatPolicyValue(workspacePolicy.monthly_budget_cents));
    setRemoteConcurrencyLimit(formatPolicyValue(workspacePolicy.remote_concurrency_limit));
    setDefaultParentIssueBudgetCents(
      formatPolicyValue(workspacePolicy.default_parent_issue_budget_cents),
    );
  }, [workspacePolicy]);

  const handleDelete = () => {
    deleteMutation.mutate(runtime.id, {
      onSuccess: () => {
        toast.success("Runtime deleted");
        setDeleteOpen(false);
      },
      onError: (e) => {
        toast.error(e instanceof Error ? e.message : "Failed to delete runtime");
      },
    });
  };

  const handleSavePolicy = async () => {
    if (!isAdmin) return;

    const monthly = Number(monthlyBudgetCents);
    const remoteConcurrency = Number(remoteConcurrencyLimit);
    const parentBudget = Number(defaultParentIssueBudgetCents);
    if (!Number.isFinite(monthly) || monthly < 0) {
      toast.error("Monthly budget must be a non-negative integer");
      return;
    }
    if (!Number.isFinite(remoteConcurrency) || remoteConcurrency < 1) {
      toast.error("Remote concurrency limit must be at least 1");
      return;
    }
    if (!Number.isFinite(parentBudget) || parentBudget < 0) {
      toast.error("Default parent issue budget must be a non-negative integer");
      return;
    }

    try {
      await policyMutation.mutateAsync({
        monthly_budget_cents: monthly,
        remote_concurrency_limit: remoteConcurrency,
        default_parent_issue_budget_cents: parentBudget,
      });
      toast.success("Runtime policy saved");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to save runtime policy");
    }
  };

  return (
    <div className="flex h-full flex-col">
      {/* Header */}
      <div className="flex h-12 shrink-0 items-center justify-between border-b px-4">
        <div className="flex min-w-0 items-center gap-2">
          <div className="flex h-7 w-7 shrink-0 items-center justify-center">
            <ProviderLogo provider={runtime.provider} className="h-5 w-5" />
          </div>
          <div className="min-w-0">
            <h2 className="text-sm font-semibold truncate">{runtime.name}</h2>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {executionState && (
            <span className="rounded border border-border/60 bg-muted px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
              {executionState}
            </span>
          )}
          <StatusBadge status={runtime.status} />
          {canDelete && (
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-muted-foreground hover:text-destructive"
              onClick={() => setDeleteOpen(true)}
            >
              <Trash2 className="h-4 w-4" />
            </Button>
          )}
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-6 space-y-6">
        {/* Info grid */}
        <div className="grid grid-cols-2 gap-4">
          <InfoField label="Runtime Mode" value={runtime.runtime_mode} />
          <InfoField label="Provider" value={runtime.provider} />
          <InfoField
            label="Status"
            value={executionState ? `${runtime.status} (${executionState})` : runtime.status}
          />
          <InfoField
            label="Last Seen"
            value={formatLastSeen(runtime.last_seen_at)}
          />
          {ownerMember && (
            <div>
              <div className="text-xs text-muted-foreground mb-1">Owner</div>
              <div className="flex items-center gap-2">
                <ActorAvatar
                  actorType="member"
                  actorId={ownerMember.user_id}
                  size={20}
                />
                <span className="text-sm">{ownerMember.name}</span>
              </div>
            </div>
          )}
          {runtime.device_info && (
            <InfoField label="Device" value={runtime.device_info} />
          )}
          {runtime.daemon_id && (
            <InfoField label="Daemon ID" value={runtime.daemon_id} mono />
          )}
        </div>

        {/* CLI Version & Update */}
        {runtime.runtime_mode === "local" && (
          <div>
            <h3 className="text-xs font-medium text-muted-foreground mb-3">
              CLI Version
            </h3>
            <UpdateSection
              runtimeId={runtime.id}
              currentVersion={cliVersion}
              isOnline={runtime.status === "online"}
              launchedBy={launchedBy}
            />
          </div>
        )}

        {/* Connection Test */}
        <div>
          <h3 className="text-xs font-medium text-muted-foreground mb-3">
            Connection Test
          </h3>
          <PingSection runtimeId={runtime.id} />
        </div>

        {/* Usage */}
        <div>
          <h3 className="text-xs font-medium text-muted-foreground mb-3">
            Token Usage
          </h3>
          <UsageSection runtimeId={runtime.id} />
        </div>

        {isAdmin ? (
          <div>
            <h3 className="text-xs font-medium text-muted-foreground mb-3">
              Workspace Runtime Policy
            </h3>
            <Card>
              <CardContent className="space-y-4 pt-6">
                {workspacePolicy ? (
                  <div className="grid gap-4 md:grid-cols-3">
                    <div>
                      <Label htmlFor="monthly-budget-cents" className="text-xs text-muted-foreground">
                        Monthly Budget (cents)
                      </Label>
                      <Input
                        id="monthly-budget-cents"
                        type="number"
                        min={0}
                        value={monthlyBudgetCents}
                        onChange={(e) => setMonthlyBudgetCents(e.target.value)}
                        className="mt-1"
                      />
                    </div>
                    <div>
                      <Label htmlFor="remote-concurrency-limit" className="text-xs text-muted-foreground">
                        Remote Concurrency Limit
                      </Label>
                      <Input
                        id="remote-concurrency-limit"
                        type="number"
                        min={1}
                        value={remoteConcurrencyLimit}
                        onChange={(e) => setRemoteConcurrencyLimit(e.target.value)}
                        className="mt-1"
                      />
                    </div>
                    <div>
                      <Label htmlFor="parent-budget-cents" className="text-xs text-muted-foreground">
                        Default Parent Issue Budget (cents)
                      </Label>
                      <Input
                        id="parent-budget-cents"
                        type="number"
                        min={0}
                        value={defaultParentIssueBudgetCents}
                        onChange={(e) => setDefaultParentIssueBudgetCents(e.target.value)}
                        className="mt-1"
                      />
                    </div>
                  </div>
                ) : (
                  <div className="text-xs text-muted-foreground">Loading runtime policy...</div>
                )}
                <div className="flex items-center gap-2">
                  <Button
                    onClick={() => void handleSavePolicy()}
                    disabled={!workspacePolicy || policyMutation.isPending}
                    size="sm"
                  >
                    {policyMutation.isPending ? (
                      <>
                        <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />
                        Saving...
                      </>
                    ) : (
                      <>
                        <Save className="mr-2 h-3.5 w-3.5" />
                        Save Policy
                      </>
                    )}
                  </Button>
                </div>
              </CardContent>
            </Card>
          </div>
        ) : (
          <div className="rounded-lg border bg-muted/20 px-4 py-3 text-xs text-muted-foreground">
            Workspace runtime policy is managed by admins.
          </div>
        )}

        {/* Metadata */}
        {runtime.metadata && Object.keys(runtime.metadata).length > 0 && (
          <div>
            <h3 className="text-xs font-medium text-muted-foreground mb-2">
              Metadata
            </h3>
            <div className="rounded-lg border bg-muted/30 p-3">
              <pre className="text-xs font-mono whitespace-pre-wrap break-all">
                {JSON.stringify(runtime.metadata, null, 2)}
              </pre>
            </div>
          </div>
        )}

        {/* Timestamps */}
        <div className="grid grid-cols-2 gap-4 border-t pt-4">
          <InfoField
            label="Created"
            value={new Date(runtime.created_at).toLocaleString()}
          />
          <InfoField
            label="Updated"
            value={new Date(runtime.updated_at).toLocaleString()}
          />
        </div>
      </div>

      {/* Delete confirmation */}
      <AlertDialog open={deleteOpen} onOpenChange={(v) => { if (!v) setDeleteOpen(false); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete Runtime</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete &ldquo;{runtime.name}&rdquo;? This action cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              onClick={handleDelete}
              disabled={deleteMutation.isPending}
            >
              {deleteMutation.isPending ? "Deleting..." : "Delete"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
