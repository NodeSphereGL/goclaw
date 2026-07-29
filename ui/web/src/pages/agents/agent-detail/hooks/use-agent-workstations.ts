import { useState, useEffect, useCallback } from "react";
import { useWs } from "@/hooks/use-ws";
import { useAuthStore } from "@/stores/use-auth-store";
import { Methods } from "@/api/protocol";
import { ApiError } from "@/api/errors";

// Matches the backend AgentWorkstationLinkView JSON shape (camelCase).
export interface AgentWorkstationLink {
  workstationId: string;
  workstationKey: string;
  name: string;
  backendType: "ssh" | "docker";
  active: boolean;
  isDefault: boolean;
}

interface WorkstationOption {
  id: string;
  workstationKey: string;
  name: string;
  backendType: "ssh" | "docker";
}

interface PermissionEntry {
  id: string;
  pattern: string;
  enabled: boolean;
}

const ALLOW_ALL_PATTERN = "**";

// Error codes that mean "workstations aren't available to this client"
// (Lite edition never registers the RPC → INVALID_REQUEST; non-admin → UNAUTHORIZED).
// In those cases the section hides itself instead of showing an error.
function isUnsupportedError(err: unknown): boolean {
  return err instanceof ApiError && (err.code === "INVALID_REQUEST" || err.code === "UNAUTHORIZED");
}

/**
 * Manages an agent's workstation links. Self-fetching and commits each action
 * immediately (not tied to the agent config-save form). Requires a valid agent
 * UUID — callers must not pass an agent_key.
 */
export function useAgentWorkstations(agentId: string) {
  const ws = useWs();
  const connected = useAuthStore((s) => s.connected);
  const [links, setLinks] = useState<AgentWorkstationLink[]>([]);
  const [available, setAvailable] = useState<WorkstationOption[]>([]);
  const [allowAll, setAllowAll] = useState<Record<string, boolean>>({});
  const [loading, setLoading] = useState(true);
  const [unsupported, setUnsupported] = useState(false);

  const load = useCallback(async () => {
    if (!connected || !agentId) return;
    setLoading(true);
    try {
      const linkRes = await ws.call<{ links: AgentWorkstationLink[] }>(
        Methods.WORKSTATIONS_LIST_LINKS,
        { agentId },
      );
      const linkList = linkRes.links ?? [];
      setLinks(linkList);

      const allRes = await ws.call<{ workstations: WorkstationOption[] }>(Methods.WORKSTATIONS_LIST);
      const linkedIds = new Set(linkList.map((l) => l.workstationId));
      setAvailable((allRes.workstations ?? []).filter((w) => !linkedIds.has(w.id)));

      // Resolve allow-all (`**`) state per linked workstation.
      const allowMap: Record<string, boolean> = {};
      await Promise.all(
        linkList.map(async (l) => {
          try {
            const permRes = await ws.call<{ permissions: PermissionEntry[] }>(
              Methods.WORKSTATIONS_PERMS_LIST,
              { workstationId: l.workstationId },
            );
            allowMap[l.workstationId] = (permRes.permissions ?? []).some(
              (p) => p.pattern === ALLOW_ALL_PATTERN && p.enabled,
            );
          } catch {
            allowMap[l.workstationId] = false;
          }
        }),
      );
      setAllowAll(allowMap);
      setUnsupported(false);
    } catch (err) {
      if (isUnsupportedError(err)) {
        setUnsupported(true);
      }
      setLinks([]);
      setAvailable([]);
    } finally {
      setLoading(false);
    }
  }, [ws, connected, agentId]);

  useEffect(() => {
    load();
  }, [load]);

  const link = useCallback(
    async (workstationId: string) => {
      await ws.call(Methods.WORKSTATIONS_LINK_AGENT, { agentId, workstationId });
      await load();
    },
    [ws, agentId, load],
  );

  const unlink = useCallback(
    async (workstationId: string) => {
      await ws.call(Methods.WORKSTATIONS_UNLINK_AGENT, { agentId, workstationId });
      await load();
    },
    [ws, agentId, load],
  );

  const setDefault = useCallback(
    async (workstationId: string) => {
      await ws.call(Methods.WORKSTATIONS_SET_DEFAULT, { agentId, workstationId });
      await load();
    },
    [ws, agentId, load],
  );

  const setAllowAllFor = useCallback(
    async (workstationId: string, on: boolean) => {
      if (on) {
        await ws.call(Methods.WORKSTATIONS_PERMS_ADD, { workstationId, pattern: ALLOW_ALL_PATTERN });
      } else {
        const permRes = await ws.call<{ permissions: PermissionEntry[] }>(
          Methods.WORKSTATIONS_PERMS_LIST,
          { workstationId },
        );
        const entry = (permRes.permissions ?? []).find((p) => p.pattern === ALLOW_ALL_PATTERN);
        if (entry) {
          await ws.call(Methods.WORKSTATIONS_PERMS_REMOVE, { id: entry.id });
        }
      }
      await load();
    },
    [ws, load],
  );

  return {
    links,
    available,
    allowAll,
    loading,
    unsupported,
    refresh: load,
    link,
    unlink,
    setDefault,
    setAllowAll: setAllowAllFor,
  };
}
