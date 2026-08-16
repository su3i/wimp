// Formats a timestamp as a coarse relative time string (e.g. "5m ago", "2h ago").
// `emptyLabel` controls what's shown when dateStr is missing (default "Never").
export function timeAgo(dateStr: string | null | undefined, emptyLabel = "Never"): string {
  if (!dateStr) return emptyLabel;
  try {
    const diff = Date.now() - new Date(dateStr).getTime();
    const mins = Math.floor(diff / 60_000);
    if (mins < 1) return "Just now";
    if (mins < 60) return `${mins}m ago`;
    const hrs = Math.floor(mins / 60);
    if (hrs < 24) return `${hrs}h ago`;
    return `${Math.floor(hrs / 24)}d ago`;
  } catch {
    return "N/A";
  }
}

// Formats a duration in seconds as a coarse uptime string (e.g. "12d 4h 30m").
// Shared by the host list and the per-host metrics tab, both of which derive it from
// the same Prometheus windows_system_boot_time_timestamp series.
export function formatUptime(seconds: number): string {
  if (!isFinite(seconds) || seconds < 0) return "N/A";
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  if (d > 0) return `${d}d ${h}h ${m}m`;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}

// Formats a timestamp as an exact, unambiguous, absolute moment (e.g.
// "Aug 9, 3:45:12 PM GMT+1") - what admins actually need to line up against other
// systems' logs, unlike the vague timeAgo(). Always in the browser's local timezone
// (never UTC), with the AM/PM and zone spelled out explicitly rather than left for the
// reader to guess or assume. Year is included only when it isn't the current year, to
// keep the common case short without silently dropping information that matters.
export function absoluteTime(dateStr: string | null | undefined, emptyLabel = "N/A"): string {
  if (!dateStr) return emptyLabel;
  try {
    const d = new Date(dateStr);
    const opts: Intl.DateTimeFormatOptions = {
      month: "short",
      day: "numeric",
      hour: "numeric",
      minute: "2-digit",
      second: "2-digit",
      hour12: true,
      timeZoneName: "short",
    };
    if (d.getFullYear() !== new Date().getFullYear()) opts.year = "numeric";
    return d.toLocaleString([], opts);
  } catch {
    return "N/A";
  }
}
