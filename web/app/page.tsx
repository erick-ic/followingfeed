import Link from "next/link";
import { redirect } from "next/navigation";
import { ArrowLeft, ArrowRight, CalendarDays, UserRound } from "lucide-react";
import { api } from "../lib/api";
import type { Article, PageResult } from "../lib/types";
import { InteractionSummary } from "../components/interaction-summary";
import { PageSelect } from "../components/page-select";
import { ArticleLink } from "../components/article-link";
import { ListScrollRestorer } from "../components/list-scroll-restorer";
import { LocalDateTime } from "../components/local-date-time";

const PAGE_SIZE = 10;
const MAX_PAGE = 1000;

export default async function Home({ searchParams }: { searchParams: Promise<{ page?: string }> }) {
  const rawPage = (await searchParams).page;
  const page = rawPage === undefined ? 1 : Number(rawPage);
  if (!Number.isSafeInteger(page) || page < 1 || page > MAX_PAGE) {
    redirect("/");
  }
  let articles: Article[] = [];
  let hasNextPage = false;
  let totalPages = 1;
  let error = "";

  try {
    const result = await api<PageResult<Article>>(`/pub/list?page=${page}&pageSize=${PAGE_SIZE}`);
    articles = result.items;
    totalPages = Math.min(MAX_PAGE, Math.max(1, result.totalPages));
    hasNextPage = page < totalPages;
  } catch (cause) {
    error = cause instanceof Error ? cause.message : "暂时无法加载文章";
  }

  // redirect 会抛出框架异常，必须放在业务错误捕获之外。
  if (!error && page > totalPages) {
    redirect(`/?page=${totalPages}`);
  }

  const emptyDescription =
    page === 1 ? "登录后写下第一篇文章，让这里开始流动。" : "返回上一页继续阅读。";

  return (
    <div className="page-shell">
      <ListScrollRestorer />
      {page === 1 && (
        <section className="hero">
          <p className="eyebrow">FollowingFeed · 技术内容社区</p>
          <h1 className="hero-title">发现值得持续关注的技术思考。</h1>
          <p className="hero-copy">
            {"阅读来自开发者的实践、工具与行业观察，"}
            {"也把你的经验整理成下一篇值得分享的文章。"}
          </p>
        </section>
      )}

      <section aria-label="文章列表">
        {error ? (
          <div className="error-state">
            <h2>文章加载失败</h2>
            <p>{error}</p>
            <Link href="/" prefetch={false} className="button secondary small">
              重新加载
            </Link>
          </div>
        ) : articles.length === 0 ? (
          <div className="empty-state">
            <h2>{page === 1 ? "还没有公开文章" : "已经到最后一页了"}</h2>
            <p>{emptyDescription}</p>
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
        )}

        {!error && (
          <nav className="pagination home-pagination" aria-label="文章分页">
            {page > 1 ? (
              <Link href={`/?page=${page - 1}`} prefetch={false} className="button secondary small">
                <ArrowLeft size={14} />
                上一页
              </Link>
            ) : (
              <span />
            )}
            <PageSelect page={page} totalPages={totalPages} basePath="/" />
            {hasNextPage ? (
              <Link href={`/?page=${page + 1}`} prefetch={false} className="button secondary small">
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
