"use client";

import { Bookmark, Eye, ThumbsUp } from "lucide-react";
import { useEffect, useState } from "react";
import { api, getInteractionStatus } from "../lib/api";

type InteractiveCounts = {
  likeCount?: number;
  readCount?: number;
  collectCount?: number;
};

type BatchListener = (counts: InteractiveCounts) => void;
const batchCache = new Map<number, InteractiveCounts>();
const batchListeners = new Map<number, Set<BatchListener>>();
let pendingIds = new Set<number>();
let batchScheduled = false;

function scheduleBatch(id: number, listener: BatchListener) {
  const cached = batchCache.get(id);
  if (cached) {
    listener(cached);
    return () => undefined;
  }
  const listeners = batchListeners.get(id) || new Set<BatchListener>();
  listeners.add(listener);
  batchListeners.set(id, listeners);
  pendingIds.add(id);
  if (!batchScheduled) {
    batchScheduled = true;
    queueMicrotask(async () => {
      const ids = [...pendingIds];
      pendingIds = new Set();
      batchScheduled = false;
      try {
        const result = await api<Record<string, InteractiveCounts>>("/pub/articles/interactions", {
          method: "POST",
          body: JSON.stringify({ ids }),
        });
        ids.forEach((articleId) => {
          const counts = result[String(articleId)] || {};
          batchCache.set(articleId, counts);
          batchListeners.get(articleId)?.forEach((notify) => notify(counts));
          batchListeners.delete(articleId);
        });
      } catch {
        ids.forEach((articleId) => {
          batchListeners.get(articleId)?.forEach((notify) => notify({}));
          batchListeners.delete(articleId);
        });
      }
    });
  }
  return () => {
    batchListeners.get(id)?.delete(listener);
  };
}

function safeCount(value: unknown) {
  const number = Number(value);
  return Number.isFinite(number) && number > 0 ? number : 0;
}

export function InteractionSummary({ articleId }: { articleId: number }) {
  const [counts, setCounts] = useState({
    likes: 0,
    reads: 0,
    collects: 0,
  });

  useEffect(() => {
    const unsubscribe = scheduleBatch(articleId, (result) => {
      setCounts({
        likes: safeCount(result.likeCount),
        reads: safeCount(result.readCount),
        collects: safeCount(result.collectCount),
      });
    });
    return unsubscribe;
  }, [articleId]);

  return (
    <span className="interaction-summary" aria-label="文章互动数据">
      <span className="article-stat" title={`${counts.likes} 个赞`}>
        <ThumbsUp size={16} strokeWidth={1.7} />
        {counts.likes > 0 && <span className="interaction-summary-count">{counts.likes}</span>}
      </span>
      <span className="article-stat" title={`${counts.reads} 次阅读`}>
        <Eye size={16} strokeWidth={1.7} />
        {counts.reads > 0 && <span className="interaction-summary-count">{counts.reads}</span>}
      </span>
      <span className="article-stat" title={`${counts.collects} 次收藏`}>
        <Bookmark size={16} strokeWidth={1.7} />
        {counts.collects > 0 && (
          <span className="interaction-summary-count">{counts.collects}</span>
        )}
      </span>
    </span>
  );
}

export function ReadStat({ articleId }: { articleId: number }) {
  const [count, setCount] = useState(0);

  useEffect(() => {
    let active = true;
    void getInteractionStatus(articleId)
      .then((result) => {
        if (active) setCount(safeCount(result.readCount));
      })
      .catch(() => {
        if (active) setCount(0);
      });
    return () => {
      active = false;
    };
  }, [articleId]);

  return (
    <span className="article-like-control static-stat" title={`${count} 次阅读`}>
      <Eye size={16} strokeWidth={1.7} />
      {count > 0 && <span className="interaction-count">{count}</span>}
    </span>
  );
}
