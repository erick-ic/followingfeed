"use client";

import { useEffect, useState } from "react";
import {
  CalendarDays,
  LoaderCircle,
  LogOut,
  Mail,
  UserRound,
} from "lucide-react";
import { AuthGuard } from "../../components/auth-guard";
import { useAuth } from "../../components/auth-provider";
import { ConfirmDialog } from "../../components/confirm-dialog";
import { api } from "../../lib/api";
import type { UserProfile } from "../../lib/types";

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

  useEffect(() => {
    void api<UserProfile>("/users/profile", {}, { auth: true })
      .then(setProfile)
      .catch((cause) => {
        setError(cause instanceof Error ? cause.message : "个人信息加载失败");
      });
  }, []);

  async function confirmLogout() {
    setLoggingOut(true);
    try {
      await logout();
      setLogoutOpen(false);
    } finally {
      setLoggingOut(false);
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
          className="button secondary small"
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
        <section className="profile-card">
          <div className="action-row">
            <div className="dialog-icon" style={{ color: "var(--accent)" }}>
              <UserRound size={20} />
            </div>
            <div>
              <p className="muted" style={{ margin: 0, fontSize: 12 }}>
                FollowingFeed 用户
              </p>
              <h2 style={{ margin: "4px 0 0" }}>{profile.nickname}</h2>
            </div>
          </div>
          <div className="profile-grid">
            <div className="profile-stat">
              <span className="meta-item">
                <Mail size={13} /> 登录邮箱
              </span>
              <strong>{profile.email}</strong>
            </div>
            <div className="profile-stat">
              <span className="meta-item">
                <CalendarDays size={13} /> 注册时间
              </span>
              <strong>
                {new Date(profile.created_at).toLocaleDateString("zh-CN")}
              </strong>
            </div>
          </div>
        </section>
      )}

      <ConfirmDialog
        open={logoutOpen}
        title="确认退出登录？"
        description="退出后需要重新登录才能继续管理、编辑和发布文章。"
        confirmLabel="确认退出"
        busyLabel="正在退出…"
        busy={loggingOut}
        onCancel={() => setLogoutOpen(false)}
        onConfirm={() => void confirmLogout()}
      />
    </div>
  );
}
