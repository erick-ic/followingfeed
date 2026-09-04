"use client";

import Link from "next/link";
import { CalendarDays, LoaderCircle, UserRound } from "lucide-react";
import { use, useCallback, useEffect, useState } from "react";
import { api } from "../../../lib/api";
import type { Article, PageResult, PublicUserProfile } from "../../../lib/types";
import { FollowButton } from "../../../components/follow-button";
import { InteractionSummary } from "../../../components/interaction-summary";
import { ArticleLink } from "../../../components/article-link";
import { ListScrollRestorer } from "../../../components/list-scroll-restorer";
import { LocalDateTime } from "../../../components/local-date-time";

const PAGE_SIZE = 20;

export default function UserPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const [profile, setProfile] = useState<PublicUserProfile | null>(null);
  const [articles, setArticles] = useState<Article[]>([]);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState("");
  const [listError, setListError] = useState("");

  useEffect(() => {
    setProfile(null);
    setArticles([]);
    setPage(1);
    setError("");
    setListError("");

    void Promise.all([
      api<PublicUserProfile>(`/pub/users/${id}/profile`),
      api<PageResult<Article>>(`/pub/users/${id}/articles?page=1&pageSize=${PAGE_SIZE}`),
    ])
      .then(([user, result]) => {
        setProfile(user);
        setArticles(result.items);
        setTotalPages(result.totalPages);
      })
      .catch((cause) => setError(cause instanceof Error ? cause.message : "用户信息加载失败"));
  }, [id]);

  const loadMore = useCallback(async () => {
    if (loadingMore || page >= totalPages) return;
    setLoadingMore(true);
    setListError("");
    try {
      const nextPage = page + 1;
      const result = await api<PageResult<Article>>(
        `/pub/users/${id}/articles?page=${nextPage}&pageSize=${PAGE_SIZE}`,
      );
      setArticles((current) => {
        const known = new Set(current.map((article) => article.id));
        return [...current, ...result.items.filter((article) => !known.has(article.id))];
      });
      setPage(result.page);
      setTotalPages(result.totalPages);
    } catch (cause) {
      setListError(cause instanceof Error ? cause.message : "更多文章加载失败");
    } finally {
      setLoadingMore(false);
    }
  }, [id, loadingMore, page, totalPages]);

  if (error) {
    return (
      <div className="page-shell">
        <div className="error-state">
          <h2>用户不存在</h2>
          <p>{error}</p>
          <Link href="/" className="button secondary small">
            返回首页
          </Link>
        </div>
      </div>
    );
  }
  if (!profile) {
    return (
      <div className="center-state">
        <LoaderCircle className="spin" size={22} />
        <span>正在加载用户信息…</span>
      </div>
    );
  }

  return (
    <div className="page-shell">
      <ListScrollRestorer />
      <section className="profile-card public-profile-card">
        <div className="public-profile-heading">
          <div className="dialog-icon">
            <UserRound size={20} />
          </div>
          <div className="public-profile-main">
            <p className="eyebrow">Author</p>
            <h1>{profile.nickname}</h1>
            <span className="meta-item">
              <CalendarDays size={13} />
              加入于 {new Date(profile.createdAt).toLocaleDateString("zh-CN")}
            </span>
          </div>
          <div className="public-profile-stats">
            <span>
              <strong>{profile.followingCount}</strong>关注
            </span>
            <span>
              <strong>{profile.followersCount}</strong>粉丝
            </span>
            <span>
              <strong>{profile.articleCount}</strong>文章
            </span>
          </div>
          <FollowButton authorId={profile.id} />
        </div>
      </section>
      <section className="public-articles">
        <div className="page-header">
          <div>
            <p className="eyebrow">Published</p>
            <h2 className="section-title">公开文章</h2>
          </div>
        </div>
        {articles.length === 0 ? (
          <div className="empty-state">
            <h2>还没有公开文章</h2>
            <p>作者暂时还没有发布文章。</p>
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
                      {article.authorNickname || profile.nickname}
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
        {listError && <p className="relation-list-status">{listError}</p>}
        {page < totalPages && (
          <div className="relation-list-status">
            <button
              type="button"
              className="button secondary small"
              disabled={loadingMore}
              onClick={() => void loadMore()}
            >
              {loadingMore ? (
                <>
                  <LoaderCircle className="spin" size={14} />
                  正在加载…
                </>
              ) : (
                "加载更多文章"
              )}
            </button>
          </div>
        )}
      </section>
    </div>
  );
}
