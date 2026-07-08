import { useCallback, useRef, useState } from "react";

const DEFAULT_COOLDOWN_MS = 10_000;

// Tracks a per-id cooldown window after a user-initiated action (restart, stop, etc.)
// so the UI can grey out that row until we have real "is this ready for another
// command" logic. Fixed duration for now, deliberately not state-driven.
export function useActionCooldown(durationMs = DEFAULT_COOLDOWN_MS) {
  const [cooling, setCooling] = useState<Set<number>>(new Set());
  const timers = useRef<Map<number, ReturnType<typeof setTimeout>>>(new Map());

  const start = useCallback(
    (id: number) => {
      setCooling((prev) => new Set(prev).add(id));
      const existing = timers.current.get(id);
      if (existing) clearTimeout(existing);
      const timer = setTimeout(() => {
        setCooling((prev) => {
          const next = new Set(prev);
          next.delete(id);
          return next;
        });
        timers.current.delete(id);
      }, durationMs);
      timers.current.set(id, timer);
    },
    [durationMs],
  );

  const isCooling = useCallback((id: number) => cooling.has(id), [cooling]);

  return { isCooling, start };
}
