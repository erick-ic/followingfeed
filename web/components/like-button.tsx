"use client";

import { ThumbsUp } from "lucide-react";
import { ArticleInteractionButton } from "./article-interaction-button";

export function LikeButton({ articleId }: { articleId: number }) {
  return (
    <ArticleInteractionButton
      articleId={articleId}
      kind="like"
      icon={ThumbsUp}
      activeLabel="取消点赞"
      inactiveLabel="点赞"
      loginLabel="登录后点赞"
      errorLabel="点赞操作失败"
    />
  );
}
