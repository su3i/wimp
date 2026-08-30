import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AlertTriangle, Check, RotateCcw, Settings as SettingsIcon } from "lucide-react";
import { toast } from "sonner";
import { InfoTooltip } from "@/components/ui/InfoTooltip";
import { usePageTitle } from "@/utils/usePageTitle";
import { cn } from "@/utils/cn";
import {
  SETTING_KEYS,
  settingService,
  type AlertSeverity,
  type Level,
  type SettingChange,
  type Settings as SettingsData,
} from "@/services/setting.service";

const LEVEL_TONE: Record<Level, string> = {
  info: "text-ink-dim",
  warning: "text-warning",
  critical: "text-danger",
  sev: "text-danger",
  disabled: "text-ink-faint",
};

// Category labels, so the severity table groups under something readable rather than the
// raw enum the API sends.
const CATEGORY_LABELS: Record<string, string> = {
  machine: "Hosts",
  apppool: "App Pools",
  iis: "Sites",
  service: "Health Checks",
  sidecar: "Sidecars",
  metrics: "Thresholds",
};

function humanizeAlertType(alertType: string) {
  return alertType.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}

function SectionCard({
  title,
  description,
  children,
}: {
  title: string;
  description: string;
  children: React.ReactNode;
}) {
  return (
    <section className='rounded-lg border border-rim bg-surface'>
      <header className='border-b border-rim px-5 py-4'>
        <h2 className='text-xs font-semibold text-ink'>{title}</h2>
        <p className='mt-1 max-w-2xl text-[0.6875rem] text-ink-faint leading-relaxed'>{description}</p>
      </header>
      <div className='px-5 py-4'>{children}</div>
    </section>
  );
}

// Marks a value the operator has changed, alongside the deployment value it replaced, so
// the chart's intent is never hidden - only overridden.
function OverrideMark({
  overridden,
  fallback,
  onReset,
}: {
  overridden: boolean;
  fallback: string;
  onReset: () => void;
}) {
  if (!overridden) {
    return <span className='text-[0.625rem] text-ink-faint'>from deployment</span>;
  }
  return (
    <button
      type='button'
      onClick={onReset}
      title={`Reset to the deployment value (${fallback})`}
      className='flex cursor-pointer items-center gap-1 text-[0.625rem] text-ink-faint transition-colors hover:text-ink'
    >
      <RotateCcw className='size-2.5' />
      overridden · reset to {fallback}
    </button>
  );
}

function Toggle({
  checked,
  onChange,
  disabled,
}: {
  checked: boolean;
  onChange: (next: boolean) => void;
  disabled?: boolean;
}) {
  return (
    <button
      type='button'
      role='switch'
      aria-checked={checked}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={cn(
        "relative h-5 w-9 shrink-0 rounded-full border transition-colors",
        disabled ? "cursor-not-allowed opacity-50" : "cursor-pointer",
        checked ? "border-primary bg-primary/30" : "border-rim bg-surface-high",
      )}
    >
      <span
        className={cn(
          "absolute top-0.5 size-3.5 rounded-full transition-all",
          checked ? "left-[18px] bg-primary" : "left-0.5 bg-ink-faint",
        )}
      />
    </button>
  );
}

function LevelSelect({
  value,
  levels,
  onChange,
  disabled,
}: {
  value: Level;
  levels: Level[];
  onChange: (next: Level) => void;
  disabled?: boolean;
}) {
  return (
    <select
      value={value}
      disabled={disabled}
      onChange={(e) => onChange(e.target.value as Level)}
      className={cn(
        "h-7 rounded-md border border-rim bg-surface px-2 text-xs capitalize focus:border-primary focus:outline-none",
        disabled ? "cursor-not-allowed opacity-50" : "cursor-pointer",
        LEVEL_TONE[value],
      )}
    >
      {levels.map((l) => (
        <option key={l} value={l} className='text-ink'>
          {l}
        </option>
      ))}
    </select>
  );
}

function ServiceRow({ name, detail, reachable, configured }: {
  name: string;
  detail: string;
  reachable: boolean;
  configured: boolean;
}) {
  const tone = !configured ? "bg-ink-faint" : reachable ? "bg-success" : "bg-danger";
  return (
    <div className='flex items-center gap-2.5 py-2'>
      <span className={cn("size-1.5 shrink-0 rounded-full", tone)} />
      <span className='text-xs text-ink'>{name}</span>
      <span className='ml-auto text-[0.6875rem] text-ink-faint'>{detail}</span>
    </div>
  );
}

function SeverityTable({
  severities,
  levels,
  onChange,
  onReset,
  saving,
}: {
  severities: AlertSeverity[];
  levels: Level[];
  onChange: (alertType: string, level: Level) => void;
  onReset: (alertType: string) => void;
  saving: boolean;
}) {
  const grouped = useMemo(() => {
    const byCategory = new Map<string, AlertSeverity[]>();
    for (const s of severities) {
      const list = byCategory.get(s.category) ?? [];
      list.push(s);
      byCategory.set(s.category, list);
    }
    return [...byCategory.entries()];
  }, [severities]);

  return (
    <div className='space-y-5'>
      {grouped.map(([category, items]) => (
        <div key={category}>
          <p className='mb-2 text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint'>
            {CATEGORY_LABELS[category] ?? category}
          </p>
          <div className='divide-y divide-rim rounded-md border border-rim'>
            {items.map((s) => (
              <div key={s.alert_type} className='flex items-center gap-3 px-3 py-2'>
                <div className='min-w-0 flex-1'>
                  <p className='truncate text-xs text-ink'>{humanizeAlertType(s.alert_type)}</p>
                  <OverrideMark
                    overridden={s.overridden}
                    fallback={s.default}
                    onReset={() => onReset(s.alert_type)}
                  />
                </div>
                <LevelSelect
                  value={s.level}
                  levels={levels}
                  disabled={saving}
                  onChange={(level) => onChange(s.alert_type, level)}
                />
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}

export function Settings() {
  usePageTitle("Settings");
  const queryClient = useQueryClient();
  const [saving, setSaving] = useState(false);

  const { data, isLoading, isError } = useQuery({
    queryKey: ["settings"],
    queryFn: settingService.get,
    // Service reachability is live state; the rest changes only when someone edits it.
    refetchInterval: 30_000,
  });

  const mutation = useMutation({
    mutationFn: (changes: SettingChange[]) => settingService.update(changes),
    onMutate: () => setSaving(true),
    onSuccess: (result) => {
      queryClient.setQueryData(["settings"], (prev: { settings: SettingsData; levels: Level[] } | undefined) =>
        prev ? { ...prev, settings: result.settings } : prev,
      );
      // /config carries enforce_mfa for the login screen.
      void queryClient.invalidateQueries({ queryKey: ["app-config"] });
      toast.success("Settings saved.");
    },
    onError: (err: unknown) => {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error ??
        "Failed to save settings.";
      toast.error(msg);
    },
    onSettled: () => setSaving(false),
  });

  function apply(changes: SettingChange[]) {
    mutation.mutate(changes);
  }

  if (isLoading) {
    return (
      <div className='space-y-4'>
        <div className='h-4 w-32 animate-pulse rounded bg-surface-high' />
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className='h-40 animate-pulse rounded-lg border border-rim bg-surface' />
        ))}
      </div>
    );
  }

  if (isError || !data) {
    return (
      <div className='flex items-center gap-2.5 rounded-lg border border-danger/20 bg-danger/5 px-4 py-3 text-xs text-danger'>
        <AlertTriangle className='size-4 shrink-0' />
        Failed to load settings. You may not have permission to view them.
      </div>
    );
  }

  const { settings, levels } = data;
  const deliveryLevels = levels.filter((l) => l !== "disabled");

  return (
    <div className='space-y-4'>
      <div>
        <h1 className='text-base font-semibold text-ink'>Settings</h1>
        <p className='mt-1 max-w-2xl text-[0.6875rem] text-ink-faint leading-relaxed'>
          Changes take effect immediately and persist across restarts. Anything left untouched
          follows the deployment's own configuration, so the Helm chart stays the source of
          truth for everything you have not explicitly changed here.
        </p>
      </div>

      <SectionCard
        title='Alerting'
        description="Outbound delivery to Alertmanager. Switching it off does not stop alerts being recorded or incidents being tracked - only the hand-off to your receivers stops."
      >
        <div className='space-y-4'>
          <div className='flex items-center gap-3'>
            <div className='min-w-0 flex-1'>
              <p className='flex items-center gap-1.5 text-xs text-ink'>
                External alerting
                <InfoTooltip text='When off, nothing is forwarded to Alertmanager. Alerts still appear in Activity and still open incidents.' />
              </p>
              <OverrideMark
                overridden={settings.alerting_enabled.overridden}
                fallback={settings.alerting_enabled.default ? "on" : "off"}
                onReset={() => apply([{ key: SETTING_KEYS.alertingEnabled, value: null }])}
              />
            </div>
            <Toggle
              checked={settings.alerting_enabled.value}
              disabled={saving}
              onChange={(next) =>
                apply([{ key: SETTING_KEYS.alertingEnabled, value: String(next) }])
              }
            />
          </div>

          <div className='flex items-center gap-3 border-t border-rim pt-4'>
            <div className='min-w-0 flex-1'>
              <p className='flex items-center gap-1.5 text-xs text-ink'>
                Minimum severity to forward
                <InfoTooltip text='Only alerts at or above this level reach Alertmanager. Everything is still recorded in-app regardless.' />
              </p>
              <OverrideMark
                overridden={settings.receiver_min_severity.overridden}
                fallback={settings.receiver_min_severity.default}
                onReset={() => apply([{ key: SETTING_KEYS.receiverMinSeverity, value: null }])}
              />
            </div>
            <LevelSelect
              value={settings.receiver_min_severity.value}
              levels={deliveryLevels}
              disabled={saving || !settings.alerting_enabled.value}
              onChange={(level) =>
                apply([{ key: SETTING_KEYS.receiverMinSeverity, value: level }])
              }
            />
          </div>
        </div>
      </SectionCard>

      <SectionCard
        title='Alert severities'
        description="What each event is worth. Setting one to disabled stops it being recorded at all, rather than merely silencing its delivery."
      >
        <SeverityTable
          severities={settings.alert_severities}
          levels={levels}
          saving={saving}
          onChange={(alertType, level) =>
            apply([{ key: SETTING_KEYS.severity(alertType), value: level }])
          }
          onReset={(alertType) => apply([{ key: SETTING_KEYS.severity(alertType), value: null }])}
        />
      </SectionCard>

      <SectionCard
        title='Security'
        description='Read-only for now. Enforcing MFA needs an enrolment step at sign-in first, or accounts without it would be refused with no way to add it.'
      >
        <div className='flex items-center justify-between gap-3 text-xs'>
          <span className='flex items-center gap-1.5 text-ink-faint'>
            Require MFA for all accounts
            <InfoTooltip text='Set in the deployment configuration. Becomes editable here once account enrolment exists.' />
          </span>
          <span className={cn(settings.enforce_mfa.value ? "text-success" : "text-ink-dim")}>
            {settings.enforce_mfa.value ? "Enabled" : "Disabled"}
          </span>
        </div>
      </SectionCard>

      <div className='grid grid-cols-1 gap-4 lg:grid-cols-2'>
        <SectionCard
          title='Agent'
          description='Read-only. The agent build every host downloads is a deployment decision, so it is set in the chart rather than here.'
        >
          <div className='divide-y divide-rim text-xs'>
            <div className='flex items-center justify-between py-2'>
              <span className='text-ink-faint'>Target version</span>
              <span className='font-mono text-ink'>{settings.agent.version}</span>
            </div>
            <div className='flex items-center justify-between py-2'>
              <span className='text-ink-faint'>Fleet auto-update</span>
              <span className={cn("flex items-center gap-1.5", settings.agent.auto_update ? "text-success" : "text-ink-dim")}>
                {settings.agent.auto_update && <Check className='size-3' />}
                {settings.agent.auto_update ? "Enabled" : "Disabled"}
              </span>
            </div>
          </div>
        </SectionCard>

        <SectionCard
          title='Services'
          description='Whether each dependency is configured and answering. Addresses and credentials are deliberately not shown.'
        >
          <div className='divide-y divide-rim'>
            {settings.services.map((s) => (
              <ServiceRow key={s.name} {...s} />
            ))}
          </div>
        </SectionCard>
      </div>

      <p className='flex items-center gap-1.5 pb-2 text-[0.625rem] text-ink-faint'>
        <SettingsIcon className='size-3' />
        Database, cache and credential configuration is not editable here by design.
      </p>
    </div>
  );
}
