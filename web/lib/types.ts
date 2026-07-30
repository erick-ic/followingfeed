export type ArticleStatus = 1 | 2 | 3;

export type Article = {
  id: number;
  title: string;
  abstract?: string;
  content?: string;
  authorId?: number;
  authorNickname?: string;
  status: ArticleStatus;
  created_at: string;
  updated_at: string;
};

export type UserProfile = {
  id: number;
  nickname: string;
  email: string;
  created_at: number;
  updated_at: number;
};

export const articleStatus: Record<
  ArticleStatus,
  { label: string; tone: "draft" | "published" | "private" }
> = {
  1: { label: "草稿", tone: "draft" },
  2: { label: "已发布", tone: "published" },
  3: { label: "私密", tone: "private" },
};
