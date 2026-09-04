"use client";

import Link from "next/link";
import { LoaderCircle, type LucideIcon } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { usePathname } from "next/navigation";
import { api, clearInteractionStatus, getInteractionStatus } from "../lib/api";
import { useAuth } from "./auth-provider";

type InteractionKind = "like" | "collect";

type Props = {
  articleId: number;
  kind: InteractionKind;
  icon: LucideIcon;
  activeLabel: string;
  inactiveLabel: string;
  loginLabel: string;
  errorLabel: string;
};

const interactionConfig = {
  like: { status: "liked", count: "likeCount", activate: "like", deactivate: "unlike" },
  collect: {
    status: "collected",
    count: "collectCount",
    activate: "collect",
    deactivate: "uncollect",
  },
} as const;

function safeCount(value: unknown) {
  const number = Number(value);
  return Number.isFinite(number) && number > 0 ? number : 0;
}

export function ArticleInteractionButton({
  articleId,
  kind,
  icon: Icon,
  activeLabel,
  inactiveLabel,
  loginLabel,
  errorLabel,
}: Props) {
  const { ready, authenticated } = useAuth();
  const pathname = usePathname();
  const config = interactionConfig[kind];
  const [active, setActive] = useState(false);
  const [count, setCount] = useState(0);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    const status = await getInteractionStatus(articleId);
    setActive(authenticated && status[config.status]);
    setCount(safeCount(status[config.count]));
  }, [articleId, authenticated, config]);

  useEffect(() => {
    if (!ready) return;
    setLoading(true);
    setError("");
    void load()
      .catch(() => {
        setActive(false);
        setCount(0);
      })
      .finally(() => setLoading(false));
  }, [ready, load]);

  async function toggle() {
    if (busy) return;
    const previousActive = active;
    const previousCount = count;
    setBusy(true);
    setError("");
    setActive(!previousActive);
    setCount(Math.max(0, previousCount + (previousActive ? -1 : 1)));
    try {
      const action = previousActive ? config.deactivate : config.activate;
      await api<null>(`/pub/articles/${articleId}/${action}`, { method: "POST" }, { auth: true });
      clearInteractionStatus(articleId);
    } catch (cause) {
      setActive(previousActive);
      setCount(previousCount);
      setError(cause instanceof Error ? cause.message : errorLabel);
    } finally {
      setBusy(false);
    }
  }

  if (!ready || loading) {
    return (
      <span className="article-like-control like-loading" aria-label="正在加载互动信息">
        <LoaderCircle className="spin" size={16} />
      </span>
    );
  }

  const label = active ? activeLabel : inactiveLabel;
  const content = (
    <>
      <Icon size={16} strokeWidth={1.8} fill={active ? "currentColor" : "none"} />
      {count > 0 && <span className="interaction-count">{count}</span>}
    </>
  );

  return (
    <span className="like-action-wrap">
      {authenticated ? (
        <button
          type="button"
          className={active ? "article-like-control active" : "article-like-control"}
          disabled={busy}
          aria-pressed={active}
          aria-label={label}
          title={label}
          onClick={() => void toggle()}
        >
          {content}
        </button>
      ) : (
        <Link
          href={`/login?next=${encodeURIComponent(pathname)}`}
          className="article-like-control"
          title={loginLabel}
          aria-label={loginLabel}
        >
          {content}
        </Link>
      )}
      {error && (
        <span className="follow-error" role="status">
          {error}
        </span>
      )}
    </span>
  );
}
