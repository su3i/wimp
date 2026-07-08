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
