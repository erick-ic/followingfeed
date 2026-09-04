"use client";

import { useLayoutEffect } from "react";
import { usePathname, useSearchParams } from "next/navigation";

const SCROLL_STATE_KEY = "followingfeed:list-scroll-state";
const RESTORE_PENDING_KEY = "followingfeed:list-restore-pending";

type ListScrollState = {
  route: string;
  articleId: number;
  viewportTop: number;
};

export function ListScrollRestorer() {
  const pathname = usePathname();
  const searchParams = useSearchParams();

  useLayoutEffect(() => {
    if (sessionStorage.getItem(RESTORE_PENDING_KEY) !== "1") return;

    sessionStorage.removeItem(RESTORE_PENDING_KEY);
    const raw = sessionStorage.getItem(SCROLL_STATE_KEY);
    if (!raw) return;

    try {
      const state = JSON.parse(raw) as ListScrollState;
      const route = `${pathname}${searchParams.size ? `?${searchParams.toString()}` : ""}`;
      if (state.route !== route) return;

      const article = document.querySelector<HTMLElement>(`[data-article-id="${state.articleId}"]`);
      if (!article) return;

      const delta = article.getBoundingClientRect().top - state.viewportTop;
      if (delta !== 0) window.scrollBy({ top: delta, left: 0, behavior: "auto" });
    } catch {
      sessionStorage.removeItem(SCROLL_STATE_KEY);
    }
  }, [pathname, searchParams]);

  return null;
}
