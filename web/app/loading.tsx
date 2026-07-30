export default function Loading() {
  return (
    <div className="page-shell route-loading" role="status" aria-live="polite">
      <span className="sr-only">正在加载页面…</span>
      <div className="skeleton skeleton-label" />
      <div className="skeleton skeleton-title" />
      <div className="skeleton skeleton-copy" />
      <div className="skeleton-list" aria-hidden="true">
        {Array.from({ length: 3 }, (_, index) => (
          <div className="skeleton-item" key={index}>
            <div className="skeleton skeleton-item-title" />
            <div className="skeleton skeleton-line" />
            <div className="skeleton skeleton-line short" />
          </div>
        ))}
      </div>
    </div>
  );
}
