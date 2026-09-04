"use client";

import Link from "next/link";
import { CalendarDays, LoaderCircle, UserRound } from "lucide-react";
import { useEffect, useState } from "react";
import { AuthGuard } from "../../components/auth-guard";
import { InteractionSummary } from "../../components/interaction-summary";
import { ArticleLink } from "../../components/article-link";
import { ListScrollRestorer } from "../../components/list-scroll-restorer";
import { PageSelect } from "../../components/page-select";
import { api } from "../../lib/api";
import type { Article, PageResult } from "../../lib/types";
import { LocalDateTime } from "../../components/local-date-time";

const PAGE_SIZE = 10;

export default function CollectionsPage() {
  return (
    <AuthGuard>
      <Collections />
    </AuthGuard>
  );
}

function Collections() {
  const [page, setPage] = useState(1);
  const [result, setResult] = useState<PageResult<Article> | null>(null);
  const [error, setError] = useState("");
  useEffect(() => {
    setError("");
    void api<PageResult<Article>>(
      `/users/me/collections?page=${page}&pageSize=${PAGE_SIZE}`,
      {},
      { auth: true },
    )
      .then(setResult)
      .catch((cause) => setError(cause instanceof Error ? cause.message : "收藏列表加载失败"));
  }, [page]);
  if (!result && !error)
    return (
      <div className="center-state">
        <LoaderCircle className="spin" size={22} />
        <span>正在加载收藏…</span>
      </div>
    );
  if (error)
    return (
      <div className="page-shell">
        <div className="error-state">
          <h2>收藏列表加载失败</h2>
          <p>{error}</p>
        </div>
      </div>
    );
  const articles = result?.items || [];
  return (
    <div className="page-shell">
      <ListScrollRestorer />
      <section className="hero">
        <p className="eyebrow">FollowingFeed · SAVED</p>
        <h1 className="hero-title">收藏的文章。</h1>
        <p className="hero-copy">
          保存感兴趣的文章，随时回来继续阅读与思考。
          {result && result.total > 0 && <span className="list-total">共 {result.total} 篇</span>}
        </p>
      </section>
      {articles.length === 0 ? (
        <div className="empty-state">
          <h2>还没有收藏文章</h2>
          <p>在文章详情页收藏感兴趣的内容。</p>
        </div>
      ) : (
        <ul className="article-list">
          {articles.map((article) => (
            <li className="article-list-item" key={article.id}>
              <ArticleLink
                href={`/articles/${article.id}`}
                articleId={article.id}
                className="article-row"
              >
                <h2>{article.title}</h2>
                <p className="article-excerpt">{article.abstract || "作者暂未提供摘要。"}</p>
                <div className="meta-row">
                  <span className="meta-item">
                    <UserRound size={16} />
                    {article.authorNickname}
                  </span>
                  <span className="meta-item">
                    <CalendarDays size={16} />
                    <LocalDateTime value={article.updatedAt || article.createdAt} />
                  </span>
                  <InteractionSummary articleId={article.id} />
                </div>
              </ArticleLink>
            </li>
          ))}
        </ul>
      )}
      {result && result.totalPages > 0 && (
        <nav className="pagination collections-pagination" aria-label="收藏分页">
          <PageSelect page={page} totalPages={result.totalPages} onChange={setPage} />
        </nav>
      )}
    </div>
  );
}
