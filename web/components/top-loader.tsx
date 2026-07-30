"use client";

import NextTopLoader from "nextjs-toploader";

export function TopLoader() {
  return (
    <NextTopLoader
      color="#e52129"
      initialPosition={0.08}
      crawlSpeed={800}
      height={2}
      crawl
      showSpinner={false}
      easing="ease"
      speed={300}
      shadow={false}
      zIndex={9999}
    />
  );
}
