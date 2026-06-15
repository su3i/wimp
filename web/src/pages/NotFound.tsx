import { useNavigate } from "react-router-dom";
import { Button } from "@/components/ui/Button";

export function NotFound() {
  const navigate = useNavigate();
  return (
    <div className="flex h-full flex-col items-center justify-center gap-4 py-20 text-center">
      <p className="text-6xl font-bold text-rim">404</p>
      <p className="text-lg font-medium text-ink">Page Not Found</p>
      <p className="text-sm text-ink-dim">The page you're looking for doesn't exist.</p>
      <Button onClick={() => navigate("/")}>Go Home</Button>
    </div>
  );
}
