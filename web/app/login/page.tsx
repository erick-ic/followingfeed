"use client";

import { FormEvent, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { LoaderCircle, LogIn } from "lucide-react";
import { useAuth } from "../../components/auth-provider";

export default function LoginPage() {
  const { login } = useAuth();
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setError("");
    setSubmitting(true);
    try {
      await login(email.trim(), password);
      router.push("/dashboard/articles");
      router.refresh();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "登录失败，请稍后重试");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="form-shell">
      <form className="form-panel" onSubmit={submit}>
        <p className="eyebrow">Welcome back</p>
        <h1>登录 FollowingFeed</h1>
        <p className="form-intro">继续管理文章，或把新的技术思考分享出去。</p>

        <div className="field">
          <label htmlFor="login-email">邮箱</label>
          <input
            id="login-email"
            type="email"
            autoComplete="email"
            placeholder="you@example.com"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            required
          />
        </div>
        <div className="field">
          <label htmlFor="login-password">密码</label>
          <input
            id="login-password"
            type="password"
            autoComplete="current-password"
            placeholder="输入你的密码"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
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
              正在登录
            </>
          ) : (
            <>
              <LogIn size={16} />
              登录
            </>
          )}
        </button>
        <p className="form-footnote">
          还没有账号？ <Link href="/register">立即注册</Link>
        </p>
      </form>
    </div>
  );
}
