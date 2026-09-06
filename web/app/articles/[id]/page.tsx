import Link from "next/link";
import { notFound } from "next/navigation";
import { CalendarDays, UserRound } from "lucide-react";
import { api, ApiError } from "../../../lib/api";
import type { Article } from "../../../lib/types";
import { MarkdownContent } from "../../../components/markdown-content";
import { FollowButton } from "../../../components/follow-button";
import { LikeButton } from "../../../components/like-button";
import { CollectButton } from "../../../components/collect-button";
import { ReadStat } from "../../../components/interaction-summary";
import { BackButton } from "../../../components/back-button";
import { ScrollActions } from "../../../components/scroll-actions";
import { DetailScrollTop } from "../../../components/detail-scroll-top";
import { LocalDateTime } from "../../../components/local-date-time";

export default async function ArticlePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  if (!/^[1-9]\d*$/.test(id)) notFound();
  const backLabel = "返回";
  let article: Article;

  try {
    article = await api<Article>(`/pub/detail/${id}`);
  } catch (cause) {
    if (cause instanceof ApiError && cause.status === 404) notFound();
    const message = cause instanceof Error ? cause.message : "文章暂时无法加载";
    return (
      <div className="reading-shell">
        <DetailScrollTop />
        <div className="error-state">
          <h1>文章暂时无法加载</h1>
          <p>{message}</p>
          <BackButton fallbackLabel={backLabel} className="button secondary small" />
        </div>
      </div>
    );
  }

  return (
    <article className="reading-shell">
      <DetailScrollTop />
      <header className="article-heading">
        <BackButton fallbackLabel={backLabel} />
        <h1 className="article-title">{article.title}</h1>
        <div className="meta-row article-detail-meta">
          {article.authorNickname && (
            <Link href={`/users/${article.authorId}`} className="meta-item article-author-meta">
              <UserRound size={16} />
              {article.authorNickname}
            </Link>
          )}
          <span className="meta-item">
            <CalendarDays size={16} />
            发布于 <LocalDateTime value={article.createdAt} />
          </span>
          {article.authorId ? <FollowButton authorId={article.authorId} /> : null}
          <LikeButton articleId={article.id} />
          <ReadStat articleId={article.id} />
          <CollectButton articleId={article.id} />
        </div>
      </header>
      <MarkdownContent content={article.content || ""} />
      <footer className="article-detail-footer">
        <span className="article-updated-at">
          最后更新于 <LocalDateTime value={article.updatedAt} />
        </span>
      </footer>
      <ScrollActions />
    </article>
  );
}
