"use client";

import { ArrowLeft } from "lucide-react";
import { useRouter } from "next/navigation";
import type { ReactNode } from "react";

export function BackButton({
  fallbackLabel = "返回",
  className = "back-link",
  icon,
}: {
  fallbackLabel?: string;
  className?: string;
  icon?: ReactNode;
}) {
  const router = useRouter();
  return (
    <button
      type="button"
      className={className}
      onClick={() => {
        if (window.history.length > 1) {
          if (sessionStorage.getItem("followingfeed:list-scroll-state")) {
            sessionStorage.setItem("followingfeed:list-restore-pending", "1");
          }
          router.back();
        } else router.push("/");
      }}
    >
      {icon || <ArrowLeft size={15} />}
      {fallbackLabel}
    </button>
  );
}
