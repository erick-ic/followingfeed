"use client";

import Link from "next/link";
import { ArrowLeft, ArrowRight, CalendarDays, LoaderCircle, UserRound } from "lucide-react";
import { useEffect, useState } from "react";
import { AuthGuard } from "../../components/auth-guard";
import { api } from "../../lib/api";
import type { Article, PageResult } from "../../lib/types";
import { InteractionSummary } from "../../components/interaction-summary";
import { ArticleLink } from "../../components/article-link";
import { ListScrollRestorer } from "../../components/list-scroll-restorer";
import { PageSelect } from "../../components/page-select";
import { LocalDateTime } from "../../components/local-date-time";

const PAGE_SIZE = 10;

export default function FeedPage() {
  return (
    <AuthGuard>
      <Feed />
    </AuthGuard>
  );
}

function Feed() {
  const [page, setPage] = useState(1);
  const [articles, setArticles] = useState<Article[]>([]);
  const [hasNext, setHasNext] = useState(false);
  const [totalPages, setTotalPages] = useState(1);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    setLoading(true);
    setError("");
    void api<PageResult<Article>>(`/feed?page=${page}&pageSize=${PAGE_SIZE}`, {}, { auth: true })
      .then((items) => {
        if (items.items.length === 0 && page > 1) {
          setPage((current) => current - 1);
          return;
        }
        setArticles(items.items);
        setHasNext(items.page < items.totalPages);
        setTotalPages(items.totalPages);
        setTotal(items.total);
      })
      .catch((cause) => setError(cause instanceof Error ? cause.message : "关注动态加载失败"))
      .finally(() => setLoading(false));
  }, [page]);

  return (
    <div className="page-shell">
      <ListScrollRestorer />
      <section className="hero">
        <p className="eyebrow">FollowingFeed · Feed</p>
        <h1 className="hero-title">来自关注作者的最新文章。</h1>
        <p className="hero-copy">
          持续关注创作者，及时看到他们分享的技术实践与思考。
          {total > 0 && <span className="list-total">共 {total} 篇</span>}
        </p>
      </section>
      {loading ? (
        <div className="center-state">
          <LoaderCircle className="spin" size={22} />
          <span>正在加载关注流…</span>
        </div>
      ) : error ? (
        <div className="error-state">
          <h2>关注动态加载失败</h2>
          <p>{error}</p>
          <button className="button secondary small" onClick={() => setPage(1)}>
            重新加载
          </button>
        </div>
      ) : articles.length === 0 ? (
        <div className="empty-state">
          <h2>{page === 1 ? "还没有关注流内容" : "已经到最后一页了"}</h2>
          <p>{page === 1 ? "去文章详情页关注感兴趣的作者吧。" : "返回上一页继续阅读。"}</p>
          <Link href="/" className="button secondary small">
            浏览最新文章
          </Link>
        </div>
      ) : (
        <>
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
                    {article.authorNickname && (
                      <span className="meta-item">
                        <UserRound size={16} />
                        {article.authorNickname}
                      </span>
                    )}
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
          <nav className="pagination" aria-label="Feed 分页">
            {page > 1 ? (
              <button className="button secondary small" onClick={() => setPage(page - 1)}>
                <ArrowLeft size={14} />
                上一页
              </button>
            ) : (
              <span />
            )}
            <PageSelect page={page} totalPages={totalPages} onChange={setPage} />
            {hasNext ? (
              <button className="button secondary small" onClick={() => setPage(page + 1)}>
                下一页
                <ArrowRight size={14} />
              </button>
            ) : (
              <span />
            )}
          </nav>
        </>
      )}
    </div>
  );
}
