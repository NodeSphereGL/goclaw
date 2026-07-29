import { useState } from "react";
import { useTranslation } from "react-i18next";
import { MonitorCog, Plus, Trash2, Star } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { toast } from "@/stores/use-toast-store";
import { userFriendlyError } from "@/lib/error-utils";
import { useAgentWorkstations, type AgentWorkstationLink } from "../hooks/use-agent-workstations";

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

interface WorkstationSectionProps {
  /** Agent UUID (NOT agent_key). Section renders nothing for a non-UUID id. */
  agentId: string;
}

export function WorkstationSection({ agentId }: WorkstationSectionProps) {
  const { t } = useTranslation("agents");
  const valid = UUID_RE.test(agentId);
  const s = "detail.workstations";

  const {
    links, available, allowAll, loading, unsupported,
    link, unlink, setDefault, setAllowAll,
  } = useAgentWorkstations(valid ? agentId : "");

  const [selected, setSelected] = useState("");
  const [unlinkTarget, setUnlinkTarget] = useState<AgentWorkstationLink | null>(null);
  const [busy, setBusy] = useState(false);

  // Gate: invalid agent id (degraded fetch) or workstations unavailable (Lite / non-admin).
  if (!valid || unsupported) return null;
  if (loading && links.length === 0) return null;

  async function run(fn: () => Promise<void>) {
    setBusy(true);
    try {
      await fn();
    } catch (err) {
      toast.error(t(`${s}.actionFailed`), userFriendlyError(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="space-y-3 rounded-lg border p-3 sm:p-4">
      <div className="flex items-center gap-2">
        <MonitorCog className="h-4 w-4 text-indigo-500 shrink-0" />
        <h3 className="text-sm font-medium">{t(`${s}.title`)}</h3>
      </div>
      <p className="text-xs text-muted-foreground">{t(`${s}.description`)}</p>

      {links.length === 0 ? (
        <p className="text-xs text-muted-foreground">{t(`${s}.empty`)}</p>
      ) : (
        <div className="space-y-2">
          {links.map((l) => (
            <div
              key={l.workstationId}
              className="flex flex-wrap items-center gap-2 rounded-md border p-2"
            >
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="truncate text-sm font-medium">{l.name || l.workstationKey}</span>
                  <Badge variant="outline" className="text-[10px]">{l.backendType}</Badge>
                  {l.isDefault && (
                    <Badge variant="default" className="text-[10px]">{t(`${s}.default`)}</Badge>
                  )}
                  {!l.active && (
                    <Badge variant="secondary" className="text-[10px]">{t(`${s}.inactive`)}</Badge>
                  )}
                </div>
                <span className="font-mono text-[11px] text-muted-foreground">{l.workstationKey}</span>
              </div>

              <div className="flex items-center gap-1.5" title={t(`${s}.allowAllHint`)}>
                <Switch
                  checked={!!allowAll[l.workstationId]}
                  disabled={busy}
                  onCheckedChange={(v) => run(() => setAllowAll(l.workstationId, v))}
                />
                <span className="text-xs text-muted-foreground">{t(`${s}.allowAll`)}</span>
              </div>

              {!l.isDefault && (
                <Button
                  variant="ghost"
                  size="sm"
                  disabled={busy}
                  onClick={() => run(() => setDefault(l.workstationId))}
                  className="gap-1"
                >
                  <Star className="h-3.5 w-3.5" />
                  {t(`${s}.setDefault`)}
                </Button>
              )}

              <Button
                variant="ghost"
                size="sm"
                disabled={busy}
                onClick={() => setUnlinkTarget(l)}
                className="gap-1 text-destructive"
              >
                <Trash2 className="h-3.5 w-3.5" />
                {t(`${s}.unlink`)}
              </Button>
            </div>
          ))}
        </div>
      )}

      {available.length > 0 && (
        <div className="flex items-center gap-2 pt-1">
          <Select value={selected} onValueChange={setSelected} disabled={busy}>
            <SelectTrigger className="flex-1">
              <SelectValue placeholder={t(`${s}.selectPlaceholder`)} />
            </SelectTrigger>
            <SelectContent>
              {available.map((w) => (
                <SelectItem key={w.id} value={w.id}>
                  {w.name || w.workstationKey}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button
            size="sm"
            disabled={busy || !selected}
            onClick={() => run(async () => { await link(selected); setSelected(""); })}
            className="gap-1"
          >
            <Plus className="h-3.5 w-3.5" />
            {t(`${s}.linkAction`)}
          </Button>
        </div>
      )}

      {unlinkTarget && (
        <ConfirmDialog
          open
          onOpenChange={() => setUnlinkTarget(null)}
          title={t(`${s}.unlinkConfirmTitle`)}
          description={
            unlinkTarget.isDefault
              ? t(`${s}.unlinkDefaultConfirm`, { name: unlinkTarget.name || unlinkTarget.workstationKey })
              : t(`${s}.unlinkConfirm`, { name: unlinkTarget.name || unlinkTarget.workstationKey })
          }
          confirmLabel={t(`${s}.unlink`)}
          variant="destructive"
          onConfirm={async () => {
            await run(() => unlink(unlinkTarget.workstationId));
            setUnlinkTarget(null);
          }}
        />
      )}
    </section>
  );
}
