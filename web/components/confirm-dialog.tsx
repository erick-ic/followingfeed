"use client";

import { AlertTriangle } from "lucide-react";

type ConfirmDialogProps = {
  open: boolean;
  title: string;
  description: string;
  confirmLabel?: string;
  busyLabel?: string;
  tone?: "default" | "danger";
  busy?: boolean;
  onCancel: () => void;
  onConfirm: () => void;
};

export function ConfirmDialog({
  open,
  title,
  description,
  confirmLabel = "确定",
  busyLabel = "处理中…",
  tone = "default",
  busy,
  onCancel,
  onConfirm,
}: ConfirmDialogProps) {
  if (!open) return null;

  return (
    <div className="dialog-backdrop" role="presentation" onMouseDown={onCancel}>
      <div
        className="dialog-panel"
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="dialog-title"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <div className="dialog-icon">
          <AlertTriangle size={20} />
        </div>
        <h2 id="dialog-title">{title}</h2>
        <p>{description}</p>
        <div className="action-row" style={{ justifyContent: "flex-end" }}>
          <button
            className="button secondary small"
            onClick={onCancel}
            disabled={busy}
          >
            取消
          </button>
          <button
            className={
              tone === "danger" ? "button danger small" : "button small"
            }
            onClick={onConfirm}
            disabled={busy}
          >
            {busy ? busyLabel : confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}
