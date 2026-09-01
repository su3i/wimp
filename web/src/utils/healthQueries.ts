// The PromQL behind application health, in one place.
//
// Uptime was previously assembled separately on the dashboard, the applications list and
// the application detail page. All three wrote the same expression, but read the result
// differently, and for any application whose health check URL had ever changed they
// disagreed - see the note on uptimeQuery below.

export const UPTIME_WINDOW = "30d";

const JOB = 'job="blackbox_http"';

// Prometheus regex matchers are fully anchored, so a pipe-joined list matches exactly
// these ids and nothing else.
export function appMatcher(appIds: number[]): string {
  return appIds
    .slice()
    .sort((a, b) => a - b)
    .join("|");
}

// Current up/down, one value per application.
//
// Aggregated rather than taken raw: an application can have more than one probe series -
// a changed health check URL leaves the old target behind - and reading whichever series
// happened to come back first gave a different answer depending on the page.
export function statusQuery(matcher: string): string {
  return `max by (application_id) (probe_success{${JOB}, application_id=~"${matcher}"})`;
}

// Uptime over the window, one value per application: successful probes divided by total
// probes.
//
// Weighted by sample count rather than averaging each series' own average. Where an
// application has several probe series in the window, a mean of means would weight a
// target that ran for two days the same as one that ran for twenty-eight. This counts
// probes, which is what uptime actually is.
export function uptimeQuery(matcher: string): string {
  const selector = `probe_success{${JOB}, application_id=~"${matcher}"}[${UPTIME_WINDOW}]`;
  return `sum by (application_id) (sum_over_time(${selector})) / sum by (application_id) (count_over_time(${selector}))`;
}

export function sslExpiryQuery(matcher: string): string {
  return `min by (application_id) (probe_ssl_earliest_cert_expiry{${JOB}, application_id=~"${matcher}"})`;
}

// Collapses an instant result keyed by application_id into a lookup, dropping anything
// unlabelled or unparseable.
export function byApplicationId(
  results: { metric: Record<string, string>; value: [number, string] }[] | undefined,
): Record<number, number> {
  const out: Record<number, number> = {};
  for (const r of results ?? []) {
    const id = Number(r.metric.application_id);
    const value = Number(r.value[1]);
    if (!Number.isNaN(id) && isFinite(value)) out[id] = value;
  }
  return out;
}
