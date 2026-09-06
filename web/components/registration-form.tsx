"use client";

import { FormEvent, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { LoaderCircle, RefreshCw, UserPlus } from "lucide-react";
import { registerRequest } from "../lib/api";
import { randomNickname } from "../lib/nicknames";

export default function RegisterPage() {
  const router = useRouter();
  const [nickname, setNickname] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    setNickname(randomNickname());
  }, []);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setError("");
    const normalizedNickname = nickname.trim();
    if (!/^[\p{Script=Han}A-Za-z0-9]{2,6}$/u.test(normalizedNickname)) {
      setError("昵称需为 2 至 6 位中文、英文字母或数字");
      return;
    }
    if (password !== confirmPassword) {
      setError("两次输入的密码不一致");
      return;
    }

    setSubmitting(true);
    try {
      await registerRequest(normalizedNickname, email.trim(), password, confirmPassword);
      router.push("/login");
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "注册失败，请稍后重试");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="form-shell">
      <form className="form-panel" onSubmit={submit}>
        <p className="eyebrow">Create account</p>
        <h1>加入 FollowingFeed</h1>
        <p className="form-intro">创建账号，开始保存草稿并发布你的技术文章。</p>

        <div className="field">
          <label htmlFor="register-nickname">昵称</label>
          <div className="input-with-action">
            <input
              id="register-nickname"
              type="text"
              autoComplete="nickname"
              maxLength={6}
              placeholder="2 至 6 位中文、字母或数字"
              value={nickname}
              onChange={(event) => setNickname(event.target.value)}
              required
            />
            <button
              type="button"
              className="input-action"
              aria-label="随机更换昵称"
              title="换一个昵称"
              onClick={() => setNickname(randomNickname())}
            >
              <RefreshCw size={15} />
              换一个
            </button>
          </div>
          <span className="field-hint">昵称用于文章署名，注册后可在个人信息中修改。</span>
        </div>

        <div className="field">
          <label htmlFor="register-email">邮箱</label>
          <input
            id="register-email"
            type="email"
            autoComplete="email"
            placeholder="you@example.com"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            required
          />
        </div>
        <div className="field">
          <label htmlFor="register-password">密码</label>
          <input
            id="register-password"
            type="password"
            autoComplete="new-password"
            placeholder="至少 8 位，包含数字和特殊字符"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            required
          />
        </div>
        <div className="field">
          <label htmlFor="register-confirm">确认密码</label>
          <input
            id="register-confirm"
            type="password"
            autoComplete="new-password"
            placeholder="再次输入密码"
            value={confirmPassword}
            onChange={(event) => setConfirmPassword(event.target.value)}
            required
          />
        </div>

        {error && (
          <p className="form-error" role="alert">
            {error}
          </p>
        )}
        <button className="button form-submit" disabled={submitting}>
          {submitting ? (
            <>
              <LoaderCircle className="spin" size={16} />
              正在注册
            </>
          ) : (
            <>
              <UserPlus size={16} />
              创建账号
            </>
          )}
        </button>
        <p className="form-footnote">
          已有账号？ <Link href="/login">返回登录</Link>
        </p>
      </form>
    </div>
  );
}
