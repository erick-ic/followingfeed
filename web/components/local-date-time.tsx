"use client";

import { useEffect, useState } from "react";

export function LocalDateTime({ value }: { value: number }) {
  const [formatted, setFormatted] = useState("");

  useEffect(() => {
    setFormatted(new Date(value).toLocaleString("zh-CN"));
  }, [value]);

  return (
    <time dateTime={new Date(value).toISOString()} suppressHydrationWarning>
      {formatted || "—"}
    </time>
  );
}
