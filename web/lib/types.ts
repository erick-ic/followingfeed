export type ArticleStatus = 1 | 2 | 3;
export type PageResult<T> = {
  items: T[];
  page: number;
  pageSize: number;
  total: number;
  totalPages: number;
};
export type MyArticleListResult = {
  list: PageResult<Article>;
  summary: { draft: number; published: number };
};

export type Article = {
  id: number;
  title: string;
  abstract?: string;
  content?: string;
  authorId?: number;
  authorNickname?: string;
  status: ArticleStatus;
  createdAt: number;
  updatedAt: number;
};

export type UserProfile = {
  id: number;
  nickname: string;
  email: string;
  createdAt: number;
  updatedAt: number;
  articleLikeCount?: number;
  articleReadCount?: number;
  articleCollectCount?: number;
};

export type PublicUserProfile = Pick<UserProfile, "id" | "nickname" | "createdAt"> & {
  followingCount: number;
  followersCount: number;
  articleCount: number;
};

export type Follow = {
  id: number;
  followerId: number;
  followingId: number;
  nickname?: string;
  createdAt: number;
  updatedAt: number;
};

export const articleStatus: Record<
  ArticleStatus,
  { label: string; tone: "draft" | "published" | "private" }
> = {
  1: { label: "草稿", tone: "draft" },
  2: { label: "已发布", tone: "published" },
  3: { label: "私密", tone: "private" },
};
