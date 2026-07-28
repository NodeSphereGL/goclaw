import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { CreateWorkstationParams } from "./hooks/use-workstations";

interface WorkstationCreateDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreate: (params: CreateWorkstationParams) => Promise<void>;
}

type BackendType = "ssh" | "docker";

export function WorkstationCreateDialog({
  open,
  onOpenChange,
  onCreate,
}: WorkstationCreateDialogProps) {
  const { t } = useTranslation("workstations");

  const [name, setName] = useState("");
  const [key, setKey] = useState("");
  const [backend, setBackend] = useState<BackendType>("ssh");
  // SSH fields — auth requires privateKey OR password (backend contract).
  const [host, setHost] = useState("");
  const [port, setPort] = useState("22");
  const [user, setUser] = useState("");
  const [privateKey, setPrivateKey] = useState("");
  const [password, setPassword] = useState("");
  const [fingerprint, setFingerprint] = useState("");
  const [rawCommand, setRawCommand] = useState(false);
  // Docker fields
  const [container, setContainer] = useState("");
  const [image, setImage] = useState("");
  const [socketPath, setSocketPath] = useState("");

  const [submitting, setSubmitting] = useState(false);
  const [fieldError, setFieldError] = useState<string | null>(null);

  function resetForm() {
    setName("");
    setKey("");
    setBackend("ssh");
    setHost("");
    setPort("22");
    setUser("");
    setPrivateKey("");
    setPassword("");
    setFingerprint("");
    setRawCommand(false);
    setContainer("");
    setImage("");
    setSocketPath("");
    setFieldError(null);
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim() || !key.trim()) return;

    // Build backend metadata using the backend camelCase contract.
    let metadata: Record<string, unknown>;
    if (backend === "ssh") {
      if (!host.trim() || !user.trim()) {
        setFieldError(t("createDialog.sshHostUserRequired"));
        return;
      }
      if (!privateKey.trim() && !password.trim()) {
        setFieldError(t("createDialog.sshAuthRequired"));
        return;
      }
      metadata = {
        host: host.trim(),
        port: parseInt(port, 10) || 22,
        user: user.trim(),
        ...(privateKey.trim() ? { privateKey: privateKey.trim() } : {}),
        ...(password.trim() ? { password: password.trim() } : {}),
        ...(fingerprint.trim() ? { knownHostsFingerprint: fingerprint.trim() } : {}),
        ...(rawCommand ? { rawCommand: true } : {}),
      };
    } else {
      if (!container.trim() && !socketPath.trim()) {
        setFieldError(t("createDialog.dockerHostRequired"));
        return;
      }
      if (!image.trim()) {
        setFieldError(t("createDialog.dockerImageRequired"));
        return;
      }
      metadata = {
        image: image.trim(),
        ...(container.trim() ? { host: container.trim() } : {}),
        ...(socketPath.trim() ? { socketPath: socketPath.trim() } : {}),
      };
    }

    setFieldError(null);
    setSubmitting(true);
    try {
      await onCreate({ workstationKey: key.trim(), name: name.trim(), backendType: backend, metadata });
      resetForm();
      onOpenChange(false);
    } catch (err) {
      setFieldError(err instanceof Error ? err.message : "Failed to create workstation.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={(v) => { if (!submitting) { resetForm(); onOpenChange(v); } }}>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{t("createDialog.title")}</DialogTitle>
            <DialogDescription>{t("createDialog.description")}</DialogDescription>
          </DialogHeader>

          <div className="mt-4 space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="ws-name">{t("createDialog.nameLabel")}</Label>
              <Input
                id="ws-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder={t("createDialog.namePlaceholder")}
                required
                className="text-base md:text-sm"
              />
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="ws-key">{t("createDialog.keyLabel")}</Label>
              <Input
                id="ws-key"
                value={key}
                onChange={(e) => setKey(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ""))}
                placeholder={t("createDialog.keyPlaceholder")}
                required
                className="text-base md:text-sm"
              />
              <p className="text-xs text-muted-foreground">{t("createDialog.keyHint")}</p>
            </div>

            <div className="space-y-1.5">
              <Label>{t("createDialog.backendLabel")}</Label>
              <Select value={backend} onValueChange={(v) => setBackend(v as BackendType)}>
                <SelectTrigger className="text-base md:text-sm">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="ssh">{t("createDialog.sshOption")}</SelectItem>
                  <SelectItem value="docker">{t("createDialog.dockerOption")}</SelectItem>
                </SelectContent>
              </Select>
            </div>

            {backend === "ssh" && (
              <>
                <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
                  <div className="space-y-1.5 sm:col-span-2">
                    <Label htmlFor="ws-host">{t("createDialog.hostLabel")}</Label>
                    <Input
                      id="ws-host"
                      value={host}
                      onChange={(e) => setHost(e.target.value)}
                      placeholder={t("createDialog.hostPlaceholder")}
                      className="text-base md:text-sm"
                    />
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="ws-port">{t("createDialog.portLabel")}</Label>
                    <Input
                      id="ws-port"
                      type="number"
                      min={1}
                      max={65535}
                      value={port}
                      onChange={(e) => setPort(e.target.value)}
                      className="text-base md:text-sm"
                    />
                  </div>
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="ws-user">{t("createDialog.userLabel")}</Label>
                  <Input
                    id="ws-user"
                    value={user}
                    onChange={(e) => setUser(e.target.value)}
                    placeholder={t("createDialog.userPlaceholder")}
                    className="text-base md:text-sm"
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="ws-privatekey">{t("createDialog.privateKeyLabel")}</Label>
                  <Textarea
                    id="ws-privatekey"
                    value={privateKey}
                    onChange={(e) => setPrivateKey(e.target.value)}
                    placeholder={t("createDialog.privateKeyPlaceholder")}
                    rows={4}
                    className="font-mono text-xs"
                  />
                  <p className="text-xs text-muted-foreground">{t("createDialog.privateKeyHint")}</p>
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="ws-password">{t("createDialog.passwordLabel")}</Label>
                  <Input
                    id="ws-password"
                    type="password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    placeholder={t("createDialog.passwordPlaceholder")}
                    className="text-base md:text-sm"
                    autoComplete="new-password"
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="ws-fingerprint">{t("createDialog.fingerprintLabel")}</Label>
                  <Input
                    id="ws-fingerprint"
                    value={fingerprint}
                    onChange={(e) => setFingerprint(e.target.value)}
                    placeholder={t("createDialog.fingerprintPlaceholder")}
                    className="text-base md:text-sm"
                  />
                  <p className="text-xs text-muted-foreground">{t("createDialog.fingerprintHint")}</p>
                </div>
                <div className="flex items-start justify-between gap-3 rounded-md border p-3">
                  <div className="space-y-0.5">
                    <Label htmlFor="ws-raw">{t("createDialog.rawCommandLabel")}</Label>
                    <p className="text-xs text-muted-foreground">{t("createDialog.rawCommandHint")}</p>
                  </div>
                  <Switch id="ws-raw" checked={rawCommand} onCheckedChange={setRawCommand} />
                </div>
              </>
            )}

            {backend === "docker" && (
              <>
                <div className="space-y-1.5">
                  <Label htmlFor="ws-container">{t("createDialog.containerLabel")}</Label>
                  <Input
                    id="ws-container"
                    value={container}
                    onChange={(e) => setContainer(e.target.value)}
                    placeholder={t("createDialog.containerPlaceholder")}
                    className="text-base md:text-sm"
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="ws-image">{t("createDialog.imageLabel")}</Label>
                  <Input
                    id="ws-image"
                    value={image}
                    onChange={(e) => setImage(e.target.value)}
                    placeholder={t("createDialog.imagePlaceholder")}
                    className="text-base md:text-sm"
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="ws-socket">{t("createDialog.socketPathLabel")}</Label>
                  <Input
                    id="ws-socket"
                    value={socketPath}
                    onChange={(e) => setSocketPath(e.target.value)}
                    placeholder={t("createDialog.socketPathPlaceholder")}
                    className="text-base md:text-sm"
                  />
                </div>
              </>
            )}

            {fieldError && (
              <p className="text-sm text-destructive">{fieldError}</p>
            )}
          </div>

          <DialogFooter className="mt-6">
            <Button type="button" variant="outline" onClick={() => { resetForm(); onOpenChange(false); }} disabled={submitting}>
              {t("createDialog.cancel")}
            </Button>
            <Button type="submit" disabled={submitting || !name.trim() || !key.trim()}>
              {t("createDialog.create")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
