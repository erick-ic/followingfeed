import { ArticleEditor } from "../../../../components/article-editor";

export default function EditArticlePage({
  params,
}: {
  params: { id: string };
}) {
  return <ArticleEditor articleId={Number(params.id)} />;
}
