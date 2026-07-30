"use client";

import { useEffect } from "react";
import { usePathname, useRouter } from "next/navigation";
import { LoaderCircle } from "lucide-react";
import { useAuth } from "./auth-provider";

export function AuthGuard({ children }: { children: React.ReactNode }) {
  const { ready, authenticated } = useAuth();
  const pathname = usePathname();
  const router = useRouter();

  useEffect(() => {
    if (ready && !authenticated) {
      router.replace(`/login?next=${encodeURIComponent(pathname)}`);
    }
  }, [ready, authenticated, pathname, router]);

  if (!ready || !authenticated) {
    return (
      <div className="center-state" role="status">
        <LoaderCircle className="spin" size={22} />
        <span>正在确认登录状态…</span>
      </div>
    );
  }

  return children;
}
