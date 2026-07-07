import { useState, useRef } from "react";
import { createPortal } from "react-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { Layers, HardDrive, Server, AlertCircle, Plus, Copy, Check, Trash2, Power, RotateCw } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { Modal } from "@/components/ui/Modal";
import { ConfirmModal } from "@/components/ui/ConfirmModal";
import { RowMenu } from "@/components/ui/RowMenu";
import { Pagination } from "@/components/ui/Pagination";
import { useProjectStore } from "@/store/project";
import { machineService } from "@/services/machine.service";
import { cn } from "@/utils/cn";
import { usePageTitle } from "@/utils/usePageTitle";
import type { Machine } from "@/types";

const PAGE_SIZE = 20;

// ── Constants ─────────────────────────────────────────────────────────────────

const FIREWALL_COMMAND =
  `New-NetFirewallRule \`\n` +
  `  -DisplayName "Windows Exporter 9182" \`\n` +
  `  -Direction Inbound \`\n` +
  `  -Protocol TCP \`\n` +
  `  -LocalPort 9182 \`\n` +
  `  -Action Allow \`\n` +
  `  -Profile Any`;

// ── Helpers ───────────────────────────────────────────────────────────────────

function filterIPv4(ips: string[]) {
  return (ips ?? []).filter((ip) => /^\d{1,3}(\.\d{1,3}){3}$/.test(ip));
}

// ── Tooltip ───────────────────────────────────────────────────────────────────

function Tooltip({ label, children }: { label: string; children: React.ReactNode }) {
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState({ top: 0, left: 0 });
  const ref = useRef<HTMLDivElement>(null);

  function show() {
    if (ref.current) {
      const rect = ref.current.getBoundingClientRect();
      setPos({ top: rect.bottom + 6, left: rect.left + rect.width / 2 });
    }
    setOpen(true);
  }

  return (
    <div ref={ref} className='inline-flex' onMouseEnter={show} onMouseLeave={() => setOpen(false)}>
      {children}
      {open &&
        createPortal(
          <div
            style={{ position: "fixed", top: pos.top, left: pos.left, transform: "translateX(-50%)" }}
            className='pointer-events-none z-[9999] px-2 py-1 rounded border border-rim bg-surface-highest text-[0.625rem] text-ink whitespace-nowrap shadow-md'
          >
            {label}
          </div>,
          document.body,
        )}
    </div>
  );
}

// function formatLastPing(lastSeenAt: string | null) {
//   if (!lastSeenAt) return "Never";
//   const diff = Date.now() - new Date(lastSeenAt).getTime();
//   const mins = Math.floor(diff / 60_000);
//   if (mins < 1) return "Just now";
//   if (mins < 60) return `${mins}m ago`;
//   const hrs = Math.floor(mins / 60);
//   if (hrs < 24) return `${hrs}h ago`;
//   return `${Math.floor(hrs / 24)}d ago`;
// }

const statusConfig: Record<string, { dot: string; text: string; label: string }> = {
  online: { dot: "bg-success", text: "text-success", label: "Online" },
  offline: { dot: "bg-danger", text: "text-danger", label: "Offline" },
  pending: { dot: "bg-warning", text: "text-warning", label: "Pending" },
  Deleting: { dot: "bg-ink-faint", text: "text-ink-faint", label: "Deleting" },
};

function IpCell({ ips }: { ips: string[] }) {
  const pillRef = useRef<HTMLSpanElement>(null);
  const [show, setShow] = useState(false);
  const [pos, setPos] = useState({ top: 0, left: 0 });

  if (!ips.length) return <span className='text-xs text-ink-faint'>N/A</span>;

  const visible = ips.slice(0, 2);
  const rest = ips.slice(2);

  function onEnter() {
    if (!pillRef.current) return;
    const r = pillRef.current.getBoundingClientRect();
    setPos({ top: r.top - 6, left: r.left });
    setShow(true);
  }

  return (
    <div className='flex items-center gap-1.5'>
      {visible.map((ip, i) => (
        <span key={i} className='font-mono text-xs text-ink-dim'>
          {ip}
        </span>
      ))}
      {rest.length > 0 && (
        <>
          <span
            ref={pillRef}
            onMouseEnter={onEnter}
            onMouseLeave={() => setShow(false)}
            className='inline-flex items-center rounded px-1.5 py-0.5 text-[0.625rem] font-medium bg-surface-high text-ink-faint border border-rim cursor-pointer select-none'
          >
            +{rest.length}
          </span>
          {show &&
            createPortal(
              <div
                style={{ position: "fixed", top: pos.top, left: pos.left, transform: "translateY(-100%)" }}
                className='z-[9999] rounded-md border border-rim bg-surface-highest shadow-[0_4px_16px_rgba(1,4,9,0.6)] px-3 py-2 space-y-1 pointer-events-none'
              >
                {rest.map((ip, i) => (
                  <p key={i} className='font-mono text-xs text-ink-dim whitespace-nowrap'>
                    {ip}
                  </p>
                ))}
              </div>,
              document.body,
            )}
        </>
      )}
    </div>
  );
}

function StatusCell({ status }: { status: string }) {
  const cfg = statusConfig[status] ?? { dot: "bg-ink-faint", text: "text-ink-faint", label: status };
  return (
    <div className='flex items-center gap-2'>
      <span className={cn("size-1.5 rounded-full shrink-0", cfg.dot)} />
      <span className={cn("text-xs font-medium", cfg.text)}>{cfg.label}</span>
    </div>
  );
}

// ── Skeleton ──────────────────────────────────────────────────────────────────

function TableSkeleton() {
  return (
    <div className='rounded-lg border border-rim overflow-hidden'>
      <div className='border-b border-rim bg-surface-alt px-5 py-2.5 flex gap-8'>
        {["w-28", "w-36", "w-20", "w-24"].map((w, i) => (
          <div key={i} className={cn("h-2.5 rounded bg-surface-high animate-pulse", w)} />
        ))}
      </div>
      {Array.from({ length: 4 }).map((_, i) => (
        <div
          key={i}
          className='flex items-center gap-6 px-5 py-4 border-b border-rim last:border-0 animate-pulse'
        >
          <div className='size-6 rounded border border-rim bg-surface-high' />
          <div className='flex-[2] h-3 w-40 rounded bg-surface-high' />
          <div className='flex-[2] h-2.5 w-36 rounded bg-surface-high' />
          <div className='flex-1 h-2.5 w-16 rounded bg-surface-high' />
          <div className='flex-1 h-2.5 w-20 rounded bg-surface-high' />
        </div>
      ))}
    </div>
  );
}

// ── Empty / placeholder states ────────────────────────────────────────────────

function EmptyState({ onAdd }: { onAdd: () => void }) {
  return (
    <div className='flex flex-col items-center justify-center py-20 rounded-lg border border-rim bg-surface text-center'>
      <div className='mb-4 flex size-11 items-center justify-center rounded-xl border border-rim bg-surface-alt'>
        <HardDrive className='size-4 text-ink-faint' />
      </div>
      <p className='text-sm font-semibold text-ink'>No hosts</p>
      <p className='mt-1 max-w-xs text-xs text-ink-faint'>No hosts are registered to this project yet.</p>
      <Button className='mt-5' onClick={onAdd} size='sm'>
        <Plus className='size-3.5' />
        New Host
      </Button>
    </div>
  );
}

function NoProjectSelected() {
  return (
    <div className='flex flex-col items-center justify-center py-24 text-center'>
      <div className='mb-4 flex size-11 items-center justify-center rounded-xl border border-rim bg-surface'>
        <Layers className='size-4 text-ink-faint' />
      </div>
      <p className='text-sm font-semibold text-ink'>No project selected</p>
      <p className='mt-1 max-w-xs text-ink-faint text-xs'>
        Select a project from the sidebar to view its hosts.
      </p>
    </div>
  );
}

// ── Page ──────────────────────────────────────────────────────────────────────

const TH = "px-5 py-2.5 text-left text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint";

export function Machines() {
  usePageTitle("Hosts");
  const { activeProject } = useProjectStore();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const [page, setPage] = useState(1);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [downloadCommand, setDownloadCommand] = useState("");
  const [runCommand, setRunCommand] = useState("");
  const [commandOpen, setCommandOpen] = useState(false);
  const [copied, setCopied] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Machine | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [commandTarget, setCommandTarget] = useState<{ machine: Machine; action: "shutdown" | "restart" } | null>(null);
  const [commandLoading, setCommandLoading] = useState(false);
  const [uninstallCmd, setUninstallCmd] = useState<string | null>(null);

  const {
    data: machinesPage,
    isLoading,
    isError,
  } = useQuery({
    queryKey: ["machines", activeProject?.Key, page],
    enabled: !!activeProject,
    refetchInterval: 5_000,
    queryFn: async () => {
      const { data } = await machineService.list(activeProject!.Key, { page, per_page: PAGE_SIZE });
      return data;
    },
  });

  async function handleConfirmAdd() {
    setCreating(true);
    try {
      const { data } = await machineService.create(activeProject!.Key);
      setDownloadCommand(data.download_command);
      setRunCommand(data.run_command);
      queryClient.invalidateQueries({ queryKey: ["machines", activeProject!.Key] });
      setConfirmOpen(false);
      setCommandOpen(true);
    } finally {
      setCreating(false);
    }
  }

  async function handleDelete() {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      const { data } = await machineService.delete(activeProject!.Key, deleteTarget.ID);
      queryClient.invalidateQueries({ queryKey: ["machines", activeProject!.Key] });
      setDeleteTarget(null);
      setUninstallCmd(data.uninstall_command);
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error ??
        "Failed to delete machine.";
      toast.error(msg);
    } finally {
      setDeleting(false);
    }
  }

  async function handleMachineCommand() {
    if (!commandTarget || !activeProject) return;
    setCommandLoading(true);
    try {
      await machineService.command(activeProject.Key, commandTarget.machine.ID, commandTarget.action);
      queryClient.invalidateQueries({ queryKey: ["machines", activeProject.Key] });
      setCommandTarget(null);
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error ??
        "Command failed. Please try again.";
      toast.error(msg);
    } finally {
      setCommandLoading(false);
    }
  }

  async function handleCopy(key: string, text: string) {
    await navigator.clipboard.writeText(text);
    setCopied(key);
    setTimeout(() => setCopied(null), 2000);
  }

  if (!activeProject) return <NoProjectSelected />;

  const machines = machinesPage?.machines ?? [];
  const totalPages = Math.max(1, Math.ceil((machinesPage?.total ?? 0) / PAGE_SIZE));
  const paginated = machines;

  return (
    <div className='space-y-5'>
      {/* Header */}
      <div className='flex items-center justify-between'>
        <div>
          <h1 className='text-base font-semibold text-ink'>Hosts</h1>
          <p className='mt-0.5 text-xs text-ink-faint'>{activeProject.Name}</p>
        </div>
        <Button size='sm' onClick={() => setConfirmOpen(true)}>
          <Plus className='size-3.5' />
          New Host
        </Button>
      </div>

      {/* Content */}
      {isLoading ? (
        <TableSkeleton />
      ) : isError ? (
        <div className='flex items-center gap-2.5 rounded-lg border border-danger/20 bg-danger/5 px-4 py-3 text-xs text-danger'>
          <AlertCircle className='size-4 shrink-0' />
          Failed to load hosts. Check your connection and try again.
        </div>
      ) : !machines.length ? (
        <EmptyState onAdd={() => setConfirmOpen(true)} />
      ) : (
        <div className='rounded-lg border border-rim overflow-hidden'>
          <div className='grid grid-cols-[2fr_2fr_1fr_auto] border-b border-rim bg-surface-alt'>
            <div className={TH}>Hostname</div>
            <div className={TH}>IP Address</div>
            <div className={TH}>Status</div>
            <div className='px-4 py-2.5' />
          </div>

          {(paginated as Machine[]).map((machine) => {
            const ipv4s = filterIPv4(machine.IPs);
            return (
              <div
                key={machine.ID}
                onClick={() => navigate(`/machines/${machine.ID}`)}
                className='grid grid-cols-[2fr_2fr_1fr_auto] items-center border-b border-rim last:border-0 hover:bg-surface-alt transition-colors duration-100 cursor-pointer'
              >
                <div className='flex items-center gap-3 px-5 py-3.5'>
                  {machine.WindowsVersion ? (
                    <Tooltip label={machine.WindowsVersion}>
                      <div className='flex size-6 shrink-0 items-center justify-center rounded border border-rim bg-surface-high'>
                        <Server className='size-3 text-ink-faint' />
                      </div>
                    </Tooltip>
                  ) : (
                    <div className='flex size-6 shrink-0 items-center justify-center rounded border border-rim bg-surface-high'>
                      <Server className='size-3 text-ink-faint' />
                    </div>
                  )}
                  {machine.Hostname ? (
                    <span className='font-mono text-xs text-ink truncate'>{machine.Hostname.toLowerCase()}</span>
                  ) : (
                    <span className='text-xs text-ink-faint italic'>Awaiting connection...</span>
                  )}
                </div>

                <div className='px-5 py-3.5'>
                  <IpCell ips={ipv4s} />
                </div>

                <div className='px-5 py-3.5'>
                  <StatusCell status={machine.Status} />
                </div>

                <div className='flex items-center justify-center px-2 py-3'>
                  <RowMenu
                    items={[
                      {
                        icon: RotateCw,
                        label: "Restart",
                        onClick: () => setCommandTarget({ machine, action: "restart" }),
                      },
                      {
                        icon: Power,
                        label: "Shutdown",
                        variant: "danger",
                        onClick: () => setCommandTarget({ machine, action: "shutdown" }),
                      },
                      { type: "separator" },
                      {
                        icon: Trash2,
                        label: "Delete",
                        variant: "danger",
                        onClick: () => setDeleteTarget(machine),
                      },
                    ]}
                  />
                </div>
              </div>
            );
          })}
        </div>
      )}

      <Pagination
        page={page}
        totalPages={totalPages}
        onPageChange={setPage}
      />

      {/* Delete confirm */}
      <ConfirmModal
        open={!!deleteTarget}
        title='Delete Host'
        description={`Are you sure you want to delete "${
          deleteTarget?.Hostname || "this host"
        }"? This cannot be undone.`}
        confirmLabel='Delete'
        loading={deleting}
        onClose={() => setDeleteTarget(null)}
        onConfirm={() => void handleDelete()}
      />

      {/* Shutdown/restart confirm */}
      <ConfirmModal
        open={!!commandTarget}
        title={commandTarget?.action === "shutdown" ? "Shutdown Host" : "Restart Host"}
        description={
          commandTarget?.action === "shutdown"
            ? `Are you sure you want to shut down "${commandTarget?.machine.Hostname || "this host"}"? It will go offline until someone powers it back on.`
            : `Are you sure you want to restart "${commandTarget?.machine.Hostname || "this host"}"? It will be briefly unreachable.`
        }
        confirmLabel={commandTarget?.action === "shutdown" ? "Shutdown" : "Restart"}
        loading={commandLoading}
        onClose={() => setCommandTarget(null)}
        onConfirm={() => void handleMachineCommand()}
      />

      {/* New machine confirm */}
      <ConfirmModal
        open={confirmOpen}
        title='New Host'
        description='Are you sure you want to connect a new host?'
        confirmLabel='Proceed'
        loading={creating}
        onClose={() => setConfirmOpen(false)}
        onConfirm={handleConfirmAdd}
      />

      {/* Bootstrap command */}
      <Modal
        open={commandOpen}
        onClose={() => setCommandOpen(false)}
        title='Bootstrap Commands'
        description='Run these in an elevated PowerShell session on the target host. Token expires in 24 hours.'
      >
        <div className='space-y-4'>
          {/* Download */}
          <div className='space-y-1.5'>
            <p className='text-xs text-ink-dim'>Step 1: Download the bootstrap script.</p>
            <div className='relative rounded-md border border-rim bg-canvas'>
              <pre className='font-mono text-xs text-ink-dim leading-relaxed px-4 py-3.5 pr-10 overflow-x-auto whitespace-pre'>
                {downloadCommand}
              </pre>
              <button
                onClick={() => handleCopy("download", downloadCommand)}
                title='Copy'
                className='cursor-pointer absolute top-2.5 right-2.5 text-ink-faint hover:text-ink transition-colors'
              >
                {copied === "download" ? (
                  <Check className='size-3.5 text-success' />
                ) : (
                  <Copy className='size-3.5' />
                )}
              </button>
            </div>
          </div>

          {/* Run */}
          <div className='space-y-1.5'>
            <p className='text-xs text-ink-dim'>
              Step 2: Execute the script to install and connect the agent.
            </p>
            <div className='relative rounded-md border border-rim bg-canvas'>
              <pre className='font-mono text-xs text-ink-dim leading-relaxed px-4 py-3.5 pr-10 overflow-x-auto whitespace-pre'>
                {runCommand}
              </pre>
              <button
                onClick={() => handleCopy("run", runCommand)}
                title='Copy'
                className='cursor-pointer absolute top-2.5 right-2.5 text-ink-faint hover:text-ink transition-colors'
              >
                {copied === "run" ? (
                  <Check className='size-3.5 text-success' />
                ) : (
                  <Copy className='size-3.5' />
                )}
              </button>
            </div>
          </div>

          {/* Firewall rule */}
          <div className='space-y-1.5'>
            <p className='text-xs text-ink-dim'>
              Step 3 (Optional): Allow Prometheus to scrape metrics from this host on port 9182.
            </p>
            <div className='relative rounded-md border border-rim bg-canvas'>
              <pre className='font-mono text-xs text-ink-dim leading-relaxed px-4 py-3.5 pr-10 overflow-x-auto whitespace-pre'>
                {FIREWALL_COMMAND}
              </pre>
              <button
                onClick={() => handleCopy("firewall", FIREWALL_COMMAND)}
                title='Copy'
                className='cursor-pointer absolute top-2.5 right-2.5 text-ink-faint hover:text-ink transition-colors'
              >
                {copied === "firewall" ? (
                  <Check className='size-3.5 text-success' />
                ) : (
                  <Copy className='size-3.5' />
                )}
              </button>
            </div>
          </div>

          <div className='flex justify-end'>
            <Button onClick={() => setCommandOpen(false)}>Done</Button>
          </div>
        </div>
      </Modal>

      {/* Uninstall command - shown after machine deletion is requested */}
      <Modal
        open={!!uninstallCmd}
        onClose={() => setUninstallCmd(null)}
        title='Uninstall Agent'
        description='Run these commands in an elevated PowerShell session on the host to remove the agent. The host will be deleted once it disconnects.'
      >
        {uninstallCmd && (
          <div className='space-y-4'>
            {uninstallCmd.split("\n").map((cmd, i) => (
              <div key={i} className='space-y-1.5'>
                <p className='text-xs text-ink-dim'>Step {i + 1}</p>
                <div className='relative rounded-md border border-rim bg-canvas'>
                  <pre className='font-mono text-xs text-ink-dim leading-relaxed px-4 py-3.5 pr-10 overflow-x-auto whitespace-pre'>
                    {cmd}
                  </pre>
                  <button
                    onClick={() => handleCopy(`uninstall-${i}`, cmd)}
                    title='Copy'
                    className='cursor-pointer absolute top-2.5 right-2.5 text-ink-faint hover:text-ink transition-colors'
                  >
                    {copied === `uninstall-${i}` ? (
                      <Check className='size-3.5 text-success' />
                    ) : (
                      <Copy className='size-3.5' />
                    )}
                  </button>
                </div>
              </div>
            ))}
            <div className='flex justify-end'>
              <Button onClick={() => setUninstallCmd(null)}>Done</Button>
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
}
