"use client";
import { useRouter } from "next/navigation";

type PaginationItem = number | "start-ellipsis" | "end-ellipsis";

function paginationItems(page: number, totalPages: number): PaginationItem[] {
  if (totalPages <= 7) {
    return Array.from({ length: totalPages }, (_, index) => index + 1);
  }
  if (page <= 4) return [1, 2, 3, 4, 5, "end-ellipsis", totalPages];
  if (page >= totalPages - 3) {
    return [
      1,
      "start-ellipsis",
      totalPages - 4,
      totalPages - 3,
      totalPages - 2,
      totalPages - 1,
      totalPages,
    ];
  }
  return [1, "start-ellipsis", page - 1, page, page + 1, "end-ellipsis", totalPages];
}

export function PageSelect({
  page,
  totalPages,
  onChange,
  basePath,
}: {
  page: number;
  totalPages: number;
  onChange?: (page: number) => void;
  basePath?: string;
}) {
  const router = useRouter();
  const lastPage = Math.max(1, totalPages);
  const currentPage = Math.min(Math.max(1, page), lastPage);
  const change = (next: number) =>
    onChange
      ? onChange(next)
      : router.push(`${basePath || ""}${basePath?.includes("?") ? "&" : "?"}page=${next}`);

  if (lastPage === 1) {
    return (
      <span className="pagination-summary" aria-current="page">
        第 1 / 1 页
      </span>
    );
  }

  return (
    <div className="pagination-pages" aria-label={`第 ${currentPage} 页，共 ${lastPage} 页`}>
      {paginationItems(currentPage, lastPage).map((item) =>
        typeof item === "number" ? (
          <button
            type="button"
            className={`pagination-page${item === currentPage ? " active" : ""}`}
            key={item}
            aria-label={item === currentPage ? `当前页，第 ${item} 页` : `前往第 ${item} 页`}
            aria-current={item === currentPage ? "page" : undefined}
            disabled={item === currentPage}
            onClick={() => change(item)}
          >
            {item}
          </button>
        ) : (
          <span className="pagination-ellipsis" aria-hidden="true" key={item}>
            …
          </span>
        ),
      )}
    </div>
  );
}
