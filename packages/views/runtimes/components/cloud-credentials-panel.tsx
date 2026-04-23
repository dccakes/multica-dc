"use client";

import { useCallback, useEffect, useState } from "react";
import { Cloud, FlaskConical, Play, RefreshCcw, Trash2 } from "lucide-react";
import { api } from "@multica/core/api";
import type { CloudRuntimeCredential } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@multica/ui/components/ui/card";
import { Input } from "@multica/ui/components/ui/input";
import { Badge } from "@multica/ui/components/ui/badge";
import { toast } from "sonner";

const DEFAULT_REGION = "iad1";

export function CloudCredentialsPanel() {
  const [items, setItems] = useState<CloudRuntimeCredential[]>([]);
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [busyId, setBusyId] = useState<string | null>(null);

  const [name, setName] = useState("");
  const [token, setToken] = useState("");
  const [projectID, setProjectID] = useState("");
  const [teamID, setTeamID] = useState("");
  const [baseSnapshotID, setBaseSnapshotID] = useState("");
  const [region, setRegion] = useState(DEFAULT_REGION);

  const load = useCallback(async () => {
    try {
      const rows = await api.listVercelCloudRuntimeCredentials();
      setItems(rows);
      setForbidden(false);
    } catch (e) {
      const msg = e instanceof Error ? e.message : "Failed to load cloud credentials";
      if (msg.toLowerCase().includes("insufficient permissions")) {
        setForbidden(true);
      } else {
        toast.error(msg);
      }
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const createCredential = async () => {
    if (!name.trim() || !token.trim() || !projectID.trim()) {
      toast.error("Name, token, and project ID are required");
      return;
    }
    setSubmitting(true);
    try {
      await api.createVercelCloudRuntimeCredential({
        name: name.trim(),
        token: token.trim(),
        project_id: projectID.trim(),
        team_id: teamID.trim() || undefined,
        base_snapshot_id: baseSnapshotID.trim() || undefined,
        region: region.trim() || DEFAULT_REGION,
      });
      setName("");
      setToken("");
      setProjectID("");
      setTeamID("");
      setBaseSnapshotID("");
      setRegion(DEFAULT_REGION);
      await load();
      toast.success("Vercel credential created");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to create credential");
    } finally {
      setSubmitting(false);
    }
  };

  const runAction = async (id: string, action: "test" | "bootstrap" | "delete" | "toggle") => {
    setBusyId(id);
    try {
      if (action === "test") {
        const res = await api.testVercelCloudRuntimeCredential(id);
        toast.success(res.message || "Credential test succeeded");
      } else if (action === "bootstrap") {
        const res = await api.bootstrapVercelCloudRuntimeCredential(id);
        toast.success(`Bootstrap ready. ${res.command}`);
      } else if (action === "delete") {
        await api.deleteVercelCloudRuntimeCredential(id);
        toast.success("Credential deleted");
      } else {
        const item = items.find((row) => row.id === id);
        if (!item) return;
        const nextStatus = item.status === "active" ? "inactive" : "active";
        await api.updateVercelCloudRuntimeCredential(id, { status: nextStatus });
        toast.success(`Credential marked ${nextStatus}`);
      }
      await load();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Action failed");
    } finally {
      setBusyId(null);
    }
  };

  if (forbidden) return null;

  return (
    <Card className="mx-4 mt-4">
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center gap-2 text-sm">
          <Cloud className="h-4 w-4 text-muted-foreground" />
          Vercel Cloud Credentials
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-2 md:grid-cols-3">
          <Input placeholder="Name" value={name} onChange={(e) => setName(e.target.value)} />
          <Input placeholder="Vercel Token" value={token} onChange={(e) => setToken(e.target.value)} />
          <Input placeholder="Project ID" value={projectID} onChange={(e) => setProjectID(e.target.value)} />
          <Input placeholder="Team ID (optional)" value={teamID} onChange={(e) => setTeamID(e.target.value)} />
          <Input
            placeholder="Base Snapshot ID (optional)"
            value={baseSnapshotID}
            onChange={(e) => setBaseSnapshotID(e.target.value)}
          />
          <Input placeholder="Region" value={region} onChange={(e) => setRegion(e.target.value)} />
        </div>
        <div className="flex items-center gap-2">
          <Button onClick={createCredential} disabled={submitting}>
            {submitting ? "Creating..." : "Add Credential"}
          </Button>
          <Button variant="outline" size="icon-sm" onClick={() => void load()} disabled={loading}>
            <RefreshCcw className="h-3.5 w-3.5" />
          </Button>
        </div>

        <div className="space-y-2">
          {loading ? (
            <div className="text-xs text-muted-foreground">Loading credentials...</div>
          ) : items.length === 0 ? (
            <div className="text-xs text-muted-foreground">No Vercel credentials configured.</div>
          ) : (
            items.map((item) => (
              <div
                key={item.id}
                className="flex flex-wrap items-center gap-2 rounded-md border px-3 py-2 text-xs"
              >
                <span className="font-medium text-foreground">{item.name}</span>
                <Badge variant={item.status === "active" ? "default" : "secondary"}>{item.status}</Badge>
                <span className="text-muted-foreground">project: {item.project_id}</span>
                <span className="text-muted-foreground">region: {item.region}</span>
                {item.base_snapshot_id && (
                  <span className="text-muted-foreground">base: {item.base_snapshot_id}</span>
                )}
                <div className="ml-auto flex items-center gap-1">
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    onClick={() => void runAction(item.id, "test")}
                    disabled={busyId === item.id}
                  >
                    <FlaskConical className="h-3.5 w-3.5" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    onClick={() => void runAction(item.id, "bootstrap")}
                    disabled={busyId === item.id}
                  >
                    <Play className="h-3.5 w-3.5" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => void runAction(item.id, "toggle")}
                    disabled={busyId === item.id}
                  >
                    {item.status === "active" ? "Disable" : "Enable"}
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    onClick={() => void runAction(item.id, "delete")}
                    disabled={busyId === item.id}
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
              </div>
            ))
          )}
        </div>
      </CardContent>
    </Card>
  );
}
