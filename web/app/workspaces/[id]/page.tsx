"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { listDocuments, createDocument, ApiError, type Document } from "@/lib/api";
import { getToken, clearToken } from "@/lib/auth";

const STATUS: Record<string, string> = {
  ready: "bg-emerald-50 text-emerald-700",
  pending: "bg-amber-50 text-amber-700",
  failed: "bg-red-50 text-red-700",
};

export default function WorkspacePage() {
  const router = useRouter();
  const { id: workspaceId } = useParams<{ id: string }>();

  const [documents, setDocuments] = useState<Document[]>([]);
  const [title, setTitle] = useState("");
  const [content, setContent] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);

  const load = useCallback(() => {
    const token = getToken();
    if (!token) {
      router.push("/login");
      return;
    }
    listDocuments(token, workspaceId)
      .then((data) => setDocuments(data.documents ?? []))
      .catch((err: unknown) => {
        if (err instanceof ApiError && err.status === 401) {
          clearToken();
          router.push("/login");
          return;
        }
        setError(err instanceof Error ? err.message : "failed to load documents");
      })
      .finally(() => setLoading(false));
  }, [router, workspaceId]);

  useEffect(() => {
    load();
  }, [load]);

  // Poll while a document is still being ingested, so status flips live.
  const hasPending = documents.some((d) => d.status === "pending");
  useEffect(() => {
    if (!hasPending) return;
    const timer = setInterval(load, 2500);
    return () => clearInterval(timer);
  }, [hasPending, load]);

  function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const token = getToken();
    if (!token || !title.trim() || !content.trim()) return;
    setSubmitting(true);
    setError("");
    createDocument(token, workspaceId, title.trim(), content.trim())
      .then((doc) => {
        setDocuments((prev) => [doc, ...prev]);
        setTitle("");
        setContent("");
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : "failed to add document"))
      .finally(() => setSubmitting(false));
  }

  function onLogout() {
    clearToken();
    router.push("/login");
  }

  return (
    <div className="min-h-screen">
      <header className="border-b border-slate-200 bg-white">
        <div className="mx-auto flex max-w-5xl items-center justify-between px-6 py-4">
          <Link href="/dashboard" className="text-lg font-bold tracking-tight">
            rag<span className="text-indigo-600">desk</span>
          </Link>
          <button
            onClick={onLogout}
            className="rounded-lg px-3 py-1.5 text-sm font-medium text-slate-600 hover:bg-slate-100"
          >
            Sign out
          </button>
        </div>
      </header>

      <main className="mx-auto max-w-5xl px-6 py-10">
        <Link href="/dashboard" className="text-sm text-slate-500 hover:text-slate-700">
          ← Workspaces
        </Link>
        <div className="mt-2 flex items-center justify-between">
          <h1 className="text-2xl font-bold text-slate-900">Documents</h1>
          <Link
            href={`/workspaces/${workspaceId}/chat`}
            className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-700"
          >
            💬 Ask your documents
          </Link>
        </div>
        <p className="mt-1 text-sm text-slate-500">
          Uploaded text is chunked, embedded and stored for retrieval. Ingestion runs asynchronously.
        </p>

        <form
          onSubmit={onSubmit}
          className="mt-6 space-y-3 rounded-2xl border border-slate-200 bg-white p-6 shadow-sm"
        >
          <input
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="Document title"
            className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200"
          />
          <textarea
            value={content}
            onChange={(e) => setContent(e.target.value)}
            placeholder="Paste the document text…"
            rows={4}
            className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200"
          />
          <div className="flex justify-end">
            <button
              type="submit"
              disabled={submitting}
              className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-700 disabled:opacity-60"
            >
              {submitting ? "Adding…" : "Add document"}
            </button>
          </div>
        </form>

        {error && <p className="mt-6 rounded-lg bg-red-50 px-4 py-2 text-sm text-red-700">{error}</p>}

        {loading ? (
          <p className="mt-10 text-slate-500">Loading…</p>
        ) : documents.length === 0 ? (
          <p className="mt-10 text-slate-500">No documents yet. Add one above to get started.</p>
        ) : (
          <ul className="mt-8 space-y-3">
            {documents.map((doc) => (
              <li
                key={doc.id}
                className="flex items-center justify-between rounded-2xl border border-slate-200 bg-white p-5 shadow-sm"
              >
                <div className="flex items-center gap-3">
                  <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-slate-100 text-slate-500">
                    📄
                  </div>
                  <div>
                    <h3 className="font-semibold text-slate-900">{doc.title}</h3>
                    <p className="text-xs text-slate-400">
                      {doc.status === "ready"
                        ? `${doc.chunk_count} chunks embedded`
                        : doc.status === "pending"
                          ? "Ingesting…"
                          : "Ingestion failed"}
                    </p>
                  </div>
                </div>
                <span
                  className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${STATUS[doc.status] ?? "bg-slate-100 text-slate-600"}`}
                >
                  {doc.status}
                </span>
              </li>
            ))}
          </ul>
        )}
      </main>
    </div>
  );
}
