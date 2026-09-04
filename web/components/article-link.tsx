"use client";

import Link from "next/link";
import type { ReactNode } from "react";
import { usePathname, useSearchParams } from "next/navigation";

const SCROLL_STATE_KEY = "followingfeed:list-scroll-state";

export function ArticleLink({
  href,
  articleId,
  children,
  className,
}: {
  href: string;
  articleId: number;
  children: ReactNode;
  className?: string;
}) {
  const pathname = usePathname();
  const searchParams = useSearchParams();

  return (
    <Link
      href={href}
      prefetch={false}
      scroll={true}
      className={className}
      data-article-id={articleId}
      onClick={(event) => {
        const route = `${pathname}${searchParams.size ? `?${searchParams.toString()}` : ""}`;
        sessionStorage.setItem(
          SCROLL_STATE_KEY,
          JSON.stringify({
            route,
            articleId,
            viewportTop: event.currentTarget.getBoundingClientRect().top,
          }),
        );
      }}
    >
      {children}
    </Link>
  );
}
