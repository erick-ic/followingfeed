"use client";

import { ArrowUp, List } from "lucide-react";
import { BackButton } from "./back-button";

export function ScrollActions() {
  return (
    <div className="article-scroll-actions">
      <BackButton
        fallbackLabel="返回列表"
        className="button secondary small"
        icon={<List size={14} />}
      />
      <button
        type="button"
        className="button secondary small"
        onClick={() => window.scrollTo({ top: 0, behavior: "smooth" })}
      >
        <ArrowUp size={14} />
        回到顶部
      </button>
    </div>
  );
}
