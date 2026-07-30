import Link from "next/link";
import { ArrowLeft, CalendarDays, UserRound } from "lucide-react";
import { api } from "../../../lib/api";
import type { Article } from "../../../lib/types";
import { MarkdownContent } from "../../../components/markdown-content";

export default async function ArticlePage({
  params,
  searchParams,
}: {
  params: { id: string };
  searchParams: { from?: string };
}) {
  const fromDashboard = searchParams.from === "dashboard";
  const backHref = fromDashboard ? "/dashboard/articles" : "/";
  const backLabel = fromDashboard ? "返回我的文章" : "返回文章列表";

  try {
    const article = await api<Article>(`/pub/detail/${params.id}`);
    return (
      <article className="reading-shell">
        <header className="article-heading">
          <Link href={backHref} className="back-link">
            <ArrowLeft size={15} />
            {backLabel}
          </Link>
          <h1 className="article-title">{article.title}</h1>
          <div className="meta-row" style={{ marginTop: 20 }}>
            {article.authorNickname && (
              <span className="meta-item">
                <UserRound size={14} />
                {article.authorNickname}
              </span>
            )}
            <span className="meta-item">
              <CalendarDays size={14} />
              更新于 {article.updated_at}
            </span>
          </div>
        </header>
        <MarkdownContent content={article.content || ""} />
      </article>
    );
  } catch (cause) {
    const message = cause instanceof Error ? cause.message : "文章暂时无法加载";
    return (
      <div className="reading-shell">
        <div className="error-state">
          <h1>没有找到这篇文章</h1>
          <p>{message}</p>
          <Link href={backHref} className="button secondary small">
            {backLabel}
          </Link>
        </div>
      </div>
    );
  }
}
