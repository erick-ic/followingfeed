"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import {
  ArrowLeft,
  ArrowRight,
  CalendarDays,
  Eye,
  FileText,
  LoaderCircle,
  PenLine,
  Send,
  Trash2,
  Undo2,
} from "lucide-react";
import { AuthGuard } from "../../../components/auth-guard";
import { ConfirmDialog } from "../../../components/confirm-dialog";
import { InteractionSummary } from "../../../components/interaction-summary";
import { PageSelect } from "../../../components/page-select";
import { LocalDateTime } from "../../../components/local-date-time";
import { api } from "../../../lib/api";
import { articleStatus, type Article, type MyArticleListResult } from "../../../lib/types";

const PAGE_SIZE = 10;

export default function MyArticlesPage() {
  return (
    <AuthGuard>
      <ArticleDashboard />
    </AuthGuard>
  );
}

function ArticleDashboard() {
  const [articles, setArticles] = useState<Article[]>([]);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [total, setTotal] = useState(0);
  const [draftCount, setDraftCount] = useState(0);
  const [publishedCount, setPublishedCount] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [loadError, setLoadError] = useState(false);
  const [workingId, setWorkingId] = useState<number | null>(null);
  const [publishTarget, setPublishTarget] = useState<Article | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Article | null>(null);
  const [withdrawTarget, setWithdrawTarget] = useState<Article | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError(false);
    setError("");
    try {
      const data = await api<MyArticleListResult>(
        `/articles/list?page=${page}&pageSize=${PAGE_SIZE}`,
        {},
        { auth: true },
      );
      setArticles(data.list.items);
      setTotalPages(data.list.totalPages);
      setTotal(data.list.total);
      setDraftCount(data.summary.draft);
      setPublishedCount(data.summary.published);
    } catch (cause) {
      setLoadError(true);
      setError(cause instanceof Error ? cause.message : "文章列表加载失败");
    } finally {
      setLoading(false);
    }
  }, [page]);

  useEffect(() => {
    void load();
  }, [load]);

  async function publish() {
    if (!publishTarget) return;
    setWorkingId(publishTarget.id);
    setError("");
    try {
      const detail = await api<Article>(`/articles/detail/${publishTarget.id}`, {}, { auth: true });
      await api<number>(
        "/articles/publish",
        {
          method: "POST",
          body: JSON.stringify({
            id: detail.id,
            title: detail.title,
            content: detail.content,
          }),
        },
        { auth: true },
      );
      setPublishTarget(null);
      await load();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "发布失败");
    } finally {
      setWorkingId(null);
    }
  }

  async function withdraw() {
    if (!withdrawTarget) return;
    setWorkingId(withdrawTarget.id);
    setError("");
    try {
      await api<number>(
        "/articles/withdraw",
        {
          method: "POST",
          body: JSON.stringify({ id: withdrawTarget.id }),
        },
        { auth: true },
      );
      setWithdrawTarget(null);
      await load();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "撤回失败");
    } finally {
      setWorkingId(null);
    }
  }

  async function remove() {
    if (!deleteTarget) return;
    setWorkingId(deleteTarget.id);
    setError("");
    try {
      await api<number>(
        "/articles/delete",
        { method: "POST", body: JSON.stringify({ id: deleteTarget.id }) },
        { auth: true },
      );
      setDeleteTarget(null);
      await load();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "删除失败");
    } finally {
      setWorkingId(null);
    }
  }

  const emptyDescription =
    page === 1 ? "从一篇草稿开始，记录值得分享的技术经验。" : "返回上一页继续查看。";

  return (
    <div className="page-shell">
      <div className="dashboard-header hero">
        <div>
          <p className="eyebrow">Writing workspace</p>
          <h1 className="hero-title">我的文章</h1>
          <p className="hero-copy">
            管理草稿与已发布内容，保持每一次更新都清晰可控。
            {total > 0 && <span className="list-total">共 {total} 篇</span>}
          </p>
          {!loading && total > 0 && (
            <div className="dashboard-article-stats" aria-label="文章状态统计">
              <span>
                <strong>{publishedCount}</strong>已发布
              </span>
              <span>
                <strong>{draftCount}</strong>草稿
              </span>
            </div>
          )}
        </div>
      </div>

      {error && !loadError && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}

      {loading ? (
        <div className="center-state">
          <LoaderCircle className="spin" size={22} />
          <span>正在加载文章…</span>
        </div>
      ) : loadError ? (
        <div className="error-state" role="alert">
          <h2>文章列表加载失败</h2>
          <p>{error}</p>
          <button className="button secondary small" onClick={() => void load()}>
            重新加载
          </button>
        </div>
      ) : articles.length === 0 ? (
        <div className="empty-state">
          <FileText size={30} />
          <h2>{page === 1 ? "还没有文章" : "这一页没有文章"}</h2>
          <p>{emptyDescription}</p>
          {page === 1 && (
            <Link href="/articles/new" className="button small">
              <PenLine size={14} />
              开始写作
            </Link>
          )}
        </div>
      ) : (
        <div>
          {articles.map((article) => {
            const status = articleStatus[article.status];
            const busy = workingId === article.id;
            return (
              <article className="management-item" key={article.id}>
                <div>
                  <h2>{article.title}</h2>
                  <p className="article-excerpt">{article.abstract || "暂无摘要"}</p>
                  <div className="meta-row">
                    <span className={`badge ${status.tone}`}>{status.label}</span>
                    <span className="meta-item">
                      <CalendarDays size={16} />
                      <LocalDateTime value={article.updatedAt} />
                    </span>
                    {article.status === 2 && <InteractionSummary articleId={article.id} />}
                  </div>
                </div>
                <div className="management-actions">
                  {article.status === 2 && (
                    <Link href={`/articles/${article.id}?from=dashboard`} className="text-button">
                      <Eye size={14} /> 查看
                    </Link>
                  )}
                  <Link href={`/articles/${article.id}/edit`} className="text-button">
                    <PenLine size={14} /> 编辑
                  </Link>
                  {article.status === 1 ? (
                    <button
                      className="text-button"
                      disabled={busy}
                      onClick={() => setPublishTarget(article)}
                    >
                      {busy ? <LoaderCircle className="spin" size={14} /> : <Send size={14} />}
                      发布
                    </button>
                  ) : article.status === 2 ? (
                    <button
                      className="text-button"
                      disabled={busy}
                      onClick={() => setWithdrawTarget(article)}
                    >
                      {busy ? <LoaderCircle className="spin" size={14} /> : <Undo2 size={14} />}
                      撤回
                    </button>
                  ) : null}
                  <button
                    className="text-button danger"
                    disabled={busy}
                    onClick={() => setDeleteTarget(article)}
                  >
                    <Trash2 size={14} /> 删除
                  </button>
                </div>
              </article>
            );
          })}
        </div>
      )}

      {!loading && (
        <nav className="pagination" aria-label="我的文章分页">
          <button
            className="button secondary small"
            disabled={page === 1}
            onClick={() => setPage((value) => Math.max(1, value - 1))}
          >
            <ArrowLeft size={14} /> 上一页
          </button>
          <PageSelect page={page} totalPages={totalPages} onChange={setPage} />
          <button
            className="button secondary small"
            disabled={page >= totalPages}
            onClick={() => setPage((value) => value + 1)}
          >
            下一页 <ArrowRight size={14} />
          </button>
        </nav>
      )}

      <ConfirmDialog
        open={Boolean(publishTarget)}
        title="发布这篇文章？"
        description="发布后文章将出现在公开文章列表中，所有访客都可以阅读。"
        confirmLabel="确认发布"
        busyLabel="正在发布…"
        busy={workingId === publishTarget?.id}
        onCancel={() => setPublishTarget(null)}
        onConfirm={() => void publish()}
      />

      <ConfirmDialog
        open={Boolean(withdrawTarget)}
        title="撤回这篇文章？"
        description="撤回后文章将不再出现在公开文章列表中，并恢复为可继续编辑的草稿。"
        confirmLabel="确认撤回"
        busyLabel="正在撤回…"
        busy={workingId === withdrawTarget?.id}
        onCancel={() => setWithdrawTarget(null)}
        onConfirm={() => void withdraw()}
      />

      <ConfirmDialog
        open={Boolean(deleteTarget)}
        title="删除这篇文章？"
        description="文章将从制作库和公开列表中移除，目前无法在前端恢复。"
        confirmLabel="确认删除"
        busyLabel="正在删除…"
        tone="danger"
        busy={workingId === deleteTarget?.id}
        onCancel={() => setDeleteTarget(null)}
        onConfirm={() => void remove()}
      />
    </div>
  );
}
