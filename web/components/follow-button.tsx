"use client";

import { BellPlus, BellRing } from "lucide-react";
import { useEffect, useState } from "react";
import { ConfirmDialog } from "./confirm-dialog";
import { getCurrentProfile, followUser, getFollowStatus, unfollowUser } from "../lib/api";
import type { UserProfile } from "../lib/types";

export function FollowButton({ authorId }: { authorId: number }) {
  const [following, setFollowing] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [busy, setBusy] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [error, setError] = useState("");
  const [ownArticle, setOwnArticle] = useState(false);

  useEffect(() => {
    void getCurrentProfile()
      .then((profile) => {
        if (profile.id === authorId) {
          setOwnArticle(true);
          setLoaded(true);
          return;
        }
        return getFollowStatus(authorId).then((result) => setFollowing(result.following));
      })
      .catch(() => undefined)
      .finally(() => setLoaded(true));
  }, [authorId]);

  async function follow() {
    setBusy(true);
    setError("");
    try {
      await followUser(authorId);
      setFollowing(true);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "关注失败");
    } finally {
      setBusy(false);
    }
  }

  async function unfollow() {
    setBusy(true);
    setError("");
    try {
      await unfollowUser(authorId);
      setFollowing(false);
      setConfirmOpen(false);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "取消关注失败");
    } finally {
      setBusy(false);
    }
  }

  if (ownArticle || !loaded || error)
    return error ? <span className="follow-error">{error}</span> : null;
  return (
    <>
      {following ? (
        <button
          className="button small follow-button following"
          disabled={busy}
          onClick={() => setConfirmOpen(true)}
        >
          <BellRing size={16} />
          已关注
        </button>
      ) : (
        <button
          className="button small follow-button"
          disabled={busy}
          onClick={() => void follow()}
        >
          <BellPlus size={16} />
          关注
        </button>
      )}
      <ConfirmDialog
        open={confirmOpen}
        title="确认取消关注？"
        description="取消后将不再收到该作者的文章更新。"
        confirmLabel="确认取消"
        busyLabel="正在取消…"
        tone="danger"
        busy={busy}
        onCancel={() => setConfirmOpen(false)}
        onConfirm={() => void unfollow()}
      />
    </>
  );
}
