"use client";

import { useEffect, useState } from "react";
import dynamic from "next/dynamic";
import { useRouter } from "next/navigation";
import {
  ArrowLeft,
  Check,
  Eye,
  FileEdit,
  LoaderCircle,
  Save,
  Send,
} from "lucide-react";
import { AuthGuard } from "./auth-guard";
import { ConfirmDialog } from "./confirm-dialog";
import { api } from "../lib/api";
import type { Article } from "../lib/types";

const MarkdownContent = dynamic(
  () => import("./markdown-content").then((module) => module.MarkdownContent),
  {
    loading: () => (
      <div className="center-state" role="status">
        <LoaderCircle className="spin" size={20} />
        <span>正在生成预览…</span>
      </div>
    ),
  },
);

export function ArticleEditor({ articleId }: { articleId?: number }) {
  return (
    <AuthGuard>
      <Editor articleId={articleId} />
    </AuthGuard>
  );
}

function Editor({ articleId: initialId }: { articleId?: number }) {
  const router = useRouter();
  const [articleId, setArticleId] = useState(initialId);
  const [title, setTitle] = useState("");
  const [content, setContent] = useState("");
  const [tab, setTab] = useState<"write" | "preview">("write");
  const [loading, setLoading] = useState(Boolean(initialId));
  const [saving, setSaving] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [leaveDialogOpen, setLeaveDialogOpen] = useState(false);
  const [publishDialogOpen, setPublishDialogOpen] = useState(false);

  useEffect(() => {
    if (!initialId) return;
    void api<Article>(`/articles/detail/${initialId}`, {}, { auth: true })
      .then((article) => {
        setTitle(article.title);
        setContent(article.content || "");
      })
      .catch((cause) => {
        setError(cause instanceof Error ? cause.message : "文章加载失败");
      })
      .finally(() => setLoading(false));
  }, [initialId]);

  useEffect(() => {
    function beforeUnload(event: BeforeUnloadEvent) {
      if (!dirty) return;
      event.preventDefault();
      event.returnValue = "";
    }
    window.addEventListener("beforeunload", beforeUnload);
    return () => window.removeEventListener("beforeunload", beforeUnload);
  }, [dirty]);

  function validate() {
    if (!title.trim()) {
      setError("请先填写文章标题");
      return false;
    }
    if (!content.trim()) {
      setError("请先填写文章正文");
      return false;
    }
    return true;
  }

  async function saveDraft() {
    if (!validate()) return;
    setSaving(true);
    setError("");
    setMessage("");
    try {
      const id = await api<number>(
        "/articles/edit",
        {
          method: "POST",
          body: JSON.stringify({
            id: articleId || 0,
            title: title.trim(),
            content,
          }),
        },
        { auth: true },
      );
      setArticleId(id);
      setDirty(false);
      setMessage("草稿已保存");
      if (!initialId) router.replace(`/articles/${id}/edit`);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "保存失败");
    } finally {
      setSaving(false);
    }
  }

  async function publish() {
    setSaving(true);
    setError("");
    setMessage("");
    try {
      const id = await api<number>(
        "/articles/publish",
        {
          method: "POST",
          body: JSON.stringify({
            id: articleId || 0,
            title: title.trim(),
            content,
          }),
        },
        { auth: true },
      );
      setDirty(false);
      setPublishDialogOpen(false);
      router.push(`/articles/${id}`);
      router.refresh();
    } catch (cause) {
      setPublishDialogOpen(false);
      setError(cause instanceof Error ? cause.message : "发布失败");
    } finally {
      setSaving(false);
    }
  }

  function requestPublish() {
    if (!validate()) return;
    setPublishDialogOpen(true);
  }

  function leaveEditor() {
    setLeaveDialogOpen(false);
    setDirty(false);
    router.push("/dashboard/articles");
  }

  if (loading) {
    return (
      <div className="center-state">
        <LoaderCircle className="spin" size={22} />
        <span>正在载入文章…</span>
      </div>
    );
  }

  return (
    <div className="wide-shell">
      <div className="editor-header">
        <div>
          <button
            type="button"
            className="back-link editor-back-button"
            style={{ marginBottom: message || dirty ? 6 : 0 }}
            onClick={() => {
              if (dirty) {
                setLeaveDialogOpen(true);
                return;
              }
              router.push("/dashboard/articles");
            }}
          >
            <ArrowLeft size={15} />
            返回我的文章
          </button>
          {(message || dirty) && (
            <div className="save-state">
              {message ? (
                <span className="meta-item">
                  <Check size={13} /> {message}
                </span>
              ) : (
                "有尚未保存的修改"
              )}
            </div>
          )}
        </div>
        <div className="editor-actions">
          <button
            className="button secondary small"
            onClick={() => void saveDraft()}
            disabled={saving}
          >
            {saving ? (
              <LoaderCircle className="spin" size={14} />
            ) : (
              <Save size={14} />
            )}
            保存草稿
          </button>
          <button
            className="button small"
            onClick={requestPublish}
            disabled={saving}
          >
            {saving ? (
              <LoaderCircle className="spin" size={14} />
            ) : (
              <Send size={14} />
            )}
            发布文章
          </button>
        </div>
      </div>

      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}

      <section className="editor-panel">
        <input
          className="title-input"
          value={title}
          onChange={(event) => {
            setTitle(event.target.value);
            setDirty(true);
            setMessage("");
          }}
          placeholder="输入文章标题"
          aria-label="文章标题"
        />
        <div className="editor-tabs" role="tablist">
          <button
            type="button"
            className={tab === "write" ? "editor-tab active" : "editor-tab"}
            onClick={() => setTab("write")}
            role="tab"
            aria-selected={tab === "write"}
          >
            <FileEdit size={14} /> 编辑
          </button>
          <button
            type="button"
            className={tab === "preview" ? "editor-tab active" : "editor-tab"}
            onClick={() => setTab("preview")}
            role="tab"
            aria-selected={tab === "preview"}
          >
            <Eye size={14} /> 预览
          </button>
        </div>
        {tab === "write" ? (
          <textarea
            className="editor-textarea"
            value={content}
            onChange={(event) => {
              setContent(event.target.value);
              setDirty(true);
              setMessage("");
            }}
            placeholder="请输入文章正文…"
            aria-label="文章正文"
          />
        ) : (
          <div className="editor-preview">
            {content.trim() ? (
              <MarkdownContent content={content} />
            ) : (
              <div className="empty-state" style={{ marginTop: 28 }}>
                <h2>还没有可预览的内容</h2>
                <p>切换到编辑模式开始写作。</p>
              </div>
            )}
          </div>
        )}
      </section>

      <ConfirmDialog
        open={leaveDialogOpen}
        title="放弃尚未保存的修改？"
        description="返回“我的文章”后，本次尚未保存的标题和正文修改将会丢失。"
        confirmLabel="放弃修改并返回"
        tone="danger"
        onCancel={() => setLeaveDialogOpen(false)}
        onConfirm={leaveEditor}
      />

      <ConfirmDialog
        open={publishDialogOpen}
        title="发布这篇文章？"
        description="发布后文章将出现在公开文章列表中，所有访客都可以阅读。"
        confirmLabel="确认发布"
        busyLabel="正在发布…"
        busy={saving}
        onCancel={() => setPublishDialogOpen(false)}
        onConfirm={() => void publish()}
      />
    </div>
  );
}
