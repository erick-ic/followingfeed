import Link from "next/link";
import { redirect } from "next/navigation";
import { ArrowLeft, ArrowRight, CalendarDays, UserRound } from "lucide-react";
import { api } from "../lib/api";
import type { Article } from "../lib/types";

const PAGE_SIZE = 10;

export default async function Home({
  searchParams,
}: {
  searchParams: { page?: string };
}) {
  const page = Math.max(1, Number(searchParams.page) || 1);
  let articles: Article[] = [];
  let hasNextPage = false;
  let error = "";

  try {
    articles = await api<Article[]>(
      `/pub/list?page=${page}&pageSize=${PAGE_SIZE}`,
    );
    if (articles.length === PAGE_SIZE) {
      try {
        const nextPage = await api<Article[]>(
          `/pub/list?page=${page + 1}&pageSize=${PAGE_SIZE}`,
        );
        hasNextPage = nextPage.length > 0;
      } catch {
        hasNextPage = false;
      }
    }
  } catch (cause) {
    error = cause instanceof Error ? cause.message : "暂时无法加载文章";
  }

  if (!error && page > 1 && articles.length === 0) {
    redirect(`/?page=${page - 1}`);
  }

  return (
    <div className="page-shell">
      {page === 1 && (
        <section className="hero">
          <p className="eyebrow">FollowingFeed · 技术内容社区</p>
          <h1 className="hero-title">发现值得持续关注的技术思考。</h1>
          <p className="hero-copy">
            阅读来自开发者的实践、工具与行业观察，也把你的经验整理成下一篇值得分享的文章。
          </p>
        </section>
      )}

      <section aria-label="文章列表">
        {error ? (
          <div className="error-state">
            <h2>文章加载失败</h2>
            <p>{error}</p>
            <Link href="/" className="button secondary small">
              重新加载
            </Link>
          </div>
        ) : articles.length === 0 ? (
          <div className="empty-state">
            <h2>{page === 1 ? "还没有公开文章" : "已经到最后一页了"}</h2>
            <p>
              {page === 1
                ? "登录后写下第一篇文章，让这里开始流动。"
                : "返回上一页继续阅读。"}
            </p>
          </div>
        ) : (
          <ul className="article-list">
            {articles.map((article) => (
              <li className="article-list-item" key={article.id}>
                <Link href={`/articles/${article.id}`} className="article-row">
                  <h2>{article.title}</h2>
                  <p className="article-excerpt">
                    {article.abstract || "作者暂未提供摘要。"}
                  </p>
                  <div className="meta-row">
                    {article.authorNickname && (
                      <span className="meta-item">
                        <UserRound size={13} />
                        {article.authorNickname}
                      </span>
                    )}
                    <span className="meta-item">
                      <CalendarDays size={13} />
                      {article.updated_at || article.created_at}
                    </span>
                  </div>
                </Link>
              </li>
            ))}
          </ul>
        )}

        {!error && (
          <nav className="pagination" aria-label="文章分页">
            {page > 1 ? (
              <Link
                href={`/?page=${page - 1}`}
                className="button secondary small"
              >
                <ArrowLeft size={14} />
                上一页
              </Link>
            ) : (
              <span />
            )}
            <span className="pagination-info">第 {page} 页</span>
            {hasNextPage ? (
              <Link
                href={`/?page=${page + 1}`}
                className="button secondary small"
              >
                下一页
                <ArrowRight size={14} />
              </Link>
            ) : (
              <span />
            )}
          </nav>
        )}
      </section>
    </div>
  );
}
