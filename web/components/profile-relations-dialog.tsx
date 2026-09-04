"use client";

import { LoaderCircle, UserRound } from "lucide-react";
import type { UIEvent } from "react";
import type { Follow } from "../lib/types";

type RelationKind = "following" | "followers";

type Props = {
  kind: RelationKind;
  items: Follow[];
  following: Follow[];
  busy: boolean;
  loading: boolean;
  page: number;
  totalPages: number;
  onClose: () => void;
  onLoadMore: () => void;
  onFollow: (userId: number) => void;
  onUnfollow: (userId: number) => void;
};

export function ProfileRelationsDialog({
  kind,
  items,
  following,
  busy,
  loading,
  page,
  totalPages,
  onClose,
  onLoadMore,
  onFollow,
  onUnfollow,
}: Props) {
  function handleScroll(event: UIEvent<HTMLDivElement>) {
    const element = event.currentTarget;
    if (element.scrollTop + element.clientHeight >= element.scrollHeight - 48) onLoadMore();
  }

  return (
    <div className="dialog-backdrop" role="presentation" onMouseDown={onClose}>
      <div
        className="relation-dialog"
        role="dialog"
        aria-modal="true"
        onMouseDown={(event) => event.stopPropagation()}
        onScroll={handleScroll}
      >
        <div className="relation-dialog-header">
          <div>
            <p className="eyebrow">Social</p>
            <h2>{kind === "following" ? "我的关注" : "我的粉丝"}</h2>
          </div>
          <button className="icon-button" onClick={onClose} aria-label="关闭">
            ×
          </button>
        </div>
        {items.length === 0 ? (
          <p className="muted">这里还没有用户</p>
        ) : (
          <ul className="relation-list">
            {items.map((item) => {
              const userId = kind === "following" ? item.followingId : item.followerId;
              const alreadyFollowing = following.some(
                (relation) => relation.followingId === userId,
              );
              return (
                <li key={item.id}>
                  <span>
                    <UserRound size={16} />
                    {item.nickname || `用户 #${userId}`}
                  </span>
                  {kind === "following" || alreadyFollowing ? (
                    <button
                      className="button secondary small"
                      disabled={busy}
                      onClick={() => onUnfollow(userId)}
                    >
                      取消关注
                    </button>
                  ) : (
                    <button
                      className="button small follow-button"
                      disabled={busy}
                      onClick={() => onFollow(userId)}
                    >
                      关注
                    </button>
                  )}
                </li>
              );
            })}
          </ul>
        )}
        {loading && (
          <p className="relation-list-status">
            <LoaderCircle className="spin" size={15} />
            正在加载…
          </p>
        )}
        {!loading && page >= totalPages && <p className="relation-list-status">没有更多了</p>}
      </div>
    </div>
  );
}
