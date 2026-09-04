import { ArticleEditor } from "../../../../components/article-editor";

export default async function EditArticlePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <ArticleEditor articleId={Number(id)} />;
}
