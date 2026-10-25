import iconSrc from "@/assets/icon.svg";

interface LoadingScreenProps {
  label?: string;
}

// Full-screen loading state. Used as the Suspense fallback for lazy-loaded routes (see
// router/index.tsx + App.tsx), which also enforce a minimum display time so this never
// flashes on screen for a few milliseconds.
export function LoadingScreen({ label = "Loading" }: LoadingScreenProps) {
  return (
    <div
      role="status"
      aria-live="polite"
      className="fixed inset-0 z-[9999] flex flex-col items-center justify-center gap-5 bg-canvas"
      style={{ animation: "loading-screen-fade-in 0.25s ease-out" }}
    >
      <img src={iconSrc} alt="" className="loading-mark-pulse size-24" />
    </div>
  );
}
