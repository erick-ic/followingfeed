"use client";

import { useCallback, useEffect, useState } from "react";
import {
  CalendarDays,
  ChartNoAxesCombined,
  Bookmark,
  Eye,
  Heart,
  LoaderCircle,
  LogOut,
  Mail,
  UsersRound,
  UserRoundCheck,
  UserRound,
} from "lucide-react";
import { AuthGuard } from "../../components/auth-guard";
import { useAuth } from "../../components/auth-provider";
import { ConfirmDialog } from "../../components/confirm-dialog";
import { ProfileRelationsDialog } from "../../components/profile-relations-dialog";
import {
  api,
  getCurrentProfile,
  followUser,
  getFollowers,
  getFollowing,
  unfollowUser,
} from "../../lib/api";
import type { Follow, UserProfile } from "../../lib/types";

export default function ProfilePage() {
  return (
    <AuthGuard>
      <Profile />
    </AuthGuard>
  );
}

function Profile() {
  const { logout } = useAuth();
  const [profile, setProfile] = useState<UserProfile | null>(null);
  const [error, setError] = useState("");
  const [logoutOpen, setLogoutOpen] = useState(false);
  const [loggingOut, setLoggingOut] = useState(false);
  const [logoutError, setLogoutError] = useState("");
  const [following, setFollowing] = useState<Follow[]>([]);
  const [followers, setFollowers] = useState<Follow[]>([]);
  const [followError, setFollowError] = useState("");
  const [followBusy, setFollowBusy] = useState(false);
  const [cancelTarget, setCancelTarget] = useState<number | null>(null);
  const [listOpen, setListOpen] = useState<"following" | "followers" | null>(null);
  const [listPage, setListPage] = useState(1);
  const [followingTotalPages, setFollowingTotalPages] = useState(1);
  const [followersTotalPages, setFollowersTotalPages] = useState(1);
  const [followingTotal, setFollowingTotal] = useState(0);
  const [followersTotal, setFollowersTotal] = useState(0);
  const [listLoading, setListLoading] = useState(false);
  const [articleStats, setArticleStats] = useState({ likes: 0, reads: 0, collections: 0 });

  useEffect(() => {
    void getCurrentProfile()
      .then((nextProfile) => {
        setProfile(nextProfile);
        setArticleStats({
          likes: nextProfile.articleLikeCount || 0,
          reads: nextProfile.articleReadCount || 0,
          collections: nextProfile.articleCollectCount || 0,
        });
      })
      .catch((cause) => {
        setError(cause instanceof Error ? cause.message : "个人信息加载失败");
      });
  }, []);

  const refreshRelations = useCallback(async () => {
    if (!profile) return;
    const [nextFollowing, nextFollowers] = await Promise.all([
      getFollowing(profile.id),
      getFollowers(profile.id),
    ]);
    setFollowing(nextFollowing.items);
    setFollowers(nextFollowers.items);
    setFollowingTotalPages(nextFollowing.totalPages);
    setFollowersTotalPages(nextFollowers.totalPages);
    setFollowingTotal(nextFollowing.total);
    setFollowersTotal(nextFollowers.total);
  }, [profile]);

  useEffect(() => {
    void refreshRelations().catch((cause) =>
      setFollowError(cause instanceof Error ? cause.message : "关注列表加载失败"),
    );
  }, [refreshRelations]);

  async function followFollower(target: number) {
    setFollowBusy(true);
    setFollowError("");
    try {
      await followUser(target);
      await refreshRelations();
    } catch (cause) {
      setFollowError(cause instanceof Error ? cause.message : "关注失败");
    } finally {
      setFollowBusy(false);
    }
  }

  async function confirmUnfollow() {
    if (cancelTarget === null) return;
    setFollowBusy(true);
    setFollowError("");
    try {
      await unfollowUser(cancelTarget);
      await refreshRelations();
      setCancelTarget(null);
    } catch (cause) {
      setFollowError(cause instanceof Error ? cause.message : "取消关注失败");
    } finally {
      setFollowBusy(false);
    }
  }

  async function confirmLogout() {
    setLoggingOut(true);
    setLogoutError("");
    try {
      await logout();
      setLogoutOpen(false);
    } catch (cause) {
      setLogoutError(cause instanceof Error ? cause.message : "退出失败，请重试");
    } finally {
      setLoggingOut(false);
    }
  }

  async function loadMoreRelations() {
    if (!profile || !listOpen || listLoading) return;
    const nextPage = listPage + 1;
    const maxPage = listOpen === "following" ? followingTotalPages : followersTotalPages;
    if (nextPage > maxPage) return;
    setListLoading(true);
    try {
      const result =
        listOpen === "following"
          ? await getFollowing(profile.id, nextPage)
          : await getFollowers(profile.id, nextPage);
      if (listOpen === "following") setFollowing((items) => [...items, ...result.items]);
      else setFollowers((items) => [...items, ...result.items]);
      setListPage(nextPage);
    } finally {
      setListLoading(false);
    }
  }

  return (
    <div className="page-shell">
      <div className="page-header">
        <div>
          <p className="eyebrow">Account</p>
          <h1 className="page-title">个人信息</h1>
        </div>
        <button
          type="button"
          className="button secondary small profile-logout-desktop"
          onClick={() => setLogoutOpen(true)}
        >
          <LogOut size={16} />
          退出登录
        </button>
      </div>

      {error ? (
        <div className="error-state">
          <h2>无法加载个人信息</h2>
          <p>{error}</p>
        </div>
      ) : !profile ? (
        <div className="center-state">
          <LoaderCircle className="spin" size={22} />
          <span>正在加载个人信息…</span>
        </div>
      ) : (
        <div className="profile-dashboard">
          <section className="profile-card profile-account-card">
            <div className="profile-identity">
              <div className="dialog-icon profile-avatar-icon" style={{ color: "var(--accent)" }}>
                <UserRound size={20} />
              </div>
              <div className="profile-identity-copy">
                <p className="muted" style={{ margin: 0, fontSize: 12 }}>
                  FollowingFeed 用户
                </p>
                <h2 style={{ margin: "4px 0 0" }}>{profile.nickname}</h2>
              </div>
            </div>
            <div className="profile-account-details">
              <div className="profile-detail-row">
                <span className="meta-item">
                  <Mail size={16} /> 登录邮箱
                </span>
                <strong>{profile.email}</strong>
              </div>
              <div className="profile-detail-row">
                <span className="meta-item">
                  <CalendarDays size={16} /> 注册时间
                </span>
                <strong>{new Date(profile.createdAt).toLocaleDateString("zh-CN")}</strong>
              </div>
            </div>
          </section>

          <section className="profile-card profile-social-card">
            <div className="profile-card-heading">
              <div className="profile-card-title-icon">
                <UsersRound size={16} />
              </div>
              <div>
                <p className="eyebrow">Social</p>
                <h2>社交关系</h2>
              </div>
            </div>
            <div className="profile-social-grid">
              <button
                className="profile-social-stat"
                onClick={() => {
                  setListPage(1);
                  setListOpen("following");
                }}
              >
                <UserRoundCheck size={16} />
                <strong>{followingTotal}</strong>
                <span>关注</span>
              </button>
              <button
                className="profile-social-stat"
                onClick={() => {
                  setListPage(1);
                  setListOpen("followers");
                }}
              >
                <UsersRound size={16} />
                <strong>{followersTotal}</strong>
                <span>粉丝</span>
              </button>
            </div>
          </section>

          <section className="profile-card profile-performance-card">
            <div className="profile-card-heading">
              <div className="profile-card-title-icon">
                <ChartNoAxesCombined size={16} />
              </div>
              <div>
                <p className="eyebrow">Content impact</p>
                <h2>内容表现</h2>
              </div>
            </div>
            <div className="profile-article-summary">
              <div>
                <Heart size={16} />
                <strong>{articleStats.likes}</strong>
                <span>文章获赞</span>
              </div>
              <div>
                <Eye size={16} />
                <strong>{articleStats.reads}</strong>
                <span>文章阅读</span>
              </div>
              <div>
                <Bookmark size={16} />
                <strong>{articleStats.collections}</strong>
                <span>文章收藏</span>
              </div>
            </div>
          </section>
        </div>
      )}

      <button
        type="button"
        className="button secondary small profile-logout-mobile"
        onClick={() => setLogoutOpen(true)}
      >
        <LogOut size={16} />
        退出登录
      </button>

      {followError && <div className="form-error follow-page-error">{followError}</div>}

      <ConfirmDialog
        open={logoutOpen}
        title="确认退出登录？"
        description={logoutError || "退出后需要重新登录才能继续管理、编辑和发布文章。"}
        confirmLabel="确认退出"
        busyLabel="正在退出…"
        busy={loggingOut}
        onCancel={() => setLogoutOpen(false)}
        onConfirm={() => void confirmLogout()}
      />
      <ConfirmDialog
        open={cancelTarget !== null}
        title="确认取消关注？"
        description={`取消后将不再收到用户 #${cancelTarget} 的文章更新。`}
        confirmLabel="确认取消"
        busyLabel="正在取消…"
        tone="danger"
        busy={followBusy}
        onCancel={() => setCancelTarget(null)}
        onConfirm={() => void confirmUnfollow()}
      />
      {listOpen && (
        <ProfileRelationsDialog
          kind={listOpen}
          items={listOpen === "following" ? following : followers}
          following={following}
          busy={followBusy}
          loading={listLoading}
          page={listPage}
          totalPages={listOpen === "following" ? followingTotalPages : followersTotalPages}
          onClose={() => setListOpen(null)}
          onLoadMore={() => void loadMoreRelations()}
          onFollow={(userId) => void followFollower(userId)}
          onUnfollow={(userId) => {
            setListOpen(null);
            setCancelTarget(userId);
          }}
        />
      )}
    </div>
  );
}
