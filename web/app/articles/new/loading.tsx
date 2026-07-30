import { LoaderCircle } from "lucide-react";

export default function LoadingNewArticle() {
  return (
    <div className="center-state" role="status">
      <LoaderCircle className="spin" size={22} />
      <span>正在打开写作空间…</span>
    </div>
  );
}
