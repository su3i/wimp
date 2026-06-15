import { Bell, Search } from "lucide-react";
import { Button } from "@/components/ui/Button";

export function Header() {
  return (
    <header className="flex h-16 items-center gap-4 border-b border-white/5 bg-[#0f0f13]/80 px-6 backdrop-blur-sm sticky top-0 z-10">
      <div className="flex flex-1 items-center gap-3 rounded-lg border border-white/5 bg-white/3 px-3 py-2 max-w-sm">
        <Search className="size-4 text-slate-500 shrink-0" />
        <input
          type="text"
          placeholder="Search..."
          className="flex-1 bg-transparent text-sm text-slate-300 placeholder:text-slate-600 outline-none"
        />
      </div>

      <div className="ml-auto flex items-center gap-2">
        <Button variant="ghost" size="sm" className="relative">
          <Bell className="size-4" />
          <span className="absolute top-1 right-1 size-1.5 rounded-full bg-indigo-500" />
        </Button>

        <div className="flex items-center gap-2.5 rounded-lg border border-white/5 bg-white/3 px-3 py-1.5">
          <div className="size-7 rounded-full bg-indigo-500/20 flex items-center justify-center text-xs font-semibold text-indigo-400">
            U
          </div>
          <div className="hidden sm:block">
            <p className="text-xs font-medium text-slate-200 leading-none">User</p>
            <p className="text-xs text-slate-500 mt-0.5">user@example.com</p>
          </div>
        </div>
      </div>
    </header>
  );
}
