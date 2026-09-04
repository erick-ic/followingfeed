"use client";

import { Bookmark } from "lucide-react";
import { ArticleInteractionButton } from "./article-interaction-button";

export function CollectButton({ articleId }: { articleId: number }) {
  return (
    <ArticleInteractionButton
      articleId={articleId}
      kind="collect"
      icon={Bookmark}
      activeLabel="取消收藏"
      inactiveLabel="收藏"
      loginLabel="登录后收藏"
      errorLabel="收藏操作失败"
    />
  );
}
