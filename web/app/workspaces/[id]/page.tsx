"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import {
  listDocuments,
  createDocument,
  deleteDocument,
  reingestDocument,
  ApiError,
  type Document,
} from "@/lib/api";
import { getToken, clearToken } from "@/lib/auth";

const STATUS: Record<string, string> = {
  ready: "bg-emerald-50 text-emerald-700",
  pending: "bg-amber-50 text-amber-700",
  failed: "bg-red-50 text-red-700",
};

// Pull plain text out of a PDF entirely in the browser — nothing but the
// extracted text is ever sent to the server. pdfjs is loaded lazily so it never
// touches the server bundle.
async function extractPdfText(file: File): Promise<string> {
  const pdfjs = await import("pdfjs-dist");
  pdfjs.GlobalWorkerOptions.workerSrc = `https://cdn.jsdelivr.net/npm/pdfjs-dist@${pdfjs.version}/build/pdf.worker.min.mjs`;
  const data = new Uint8Array(await file.arrayBuffer());
  const pdf = await pdfjs.getDocument({ data }).promise;
  const pages: string[] = [];
  for (let i = 1; i <= pdf.numPages; i++) {
    const page = await pdf.getPage(i);
    const content = await page.getTextContent();
    pages.push(content.items.map((it) => ("str" in it ? it.str : "")).join(" "));
  }
  return pages.join("\n\n").trim();
}

export default function WorkspacePage() {
  const router = useRouter();
  const { id: workspaceId } = useParams<{ id: string }>();
  const fileRef = useRef<HTMLInputElement>(null);

  const [documents, setDocuments] = useState<Document[]>([]);
  const [title, setTitle] = useState("");
  const [content, setContent] = useState("");
  const [fileName, setFileName] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [reading, setReading] = useState(false);
  const [dragging, setDragging] = useState(false);
  const [busyId, setBusyId] = useState("");

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

  async function handleFile(file: File) {
    setError("");
    setReading(true);
    setFileName(file.name);
    try {
      const isPdf = file.type === "application/pdf" || file.name.toLowerCase().endsWith(".pdf");
      const text = isPdf ? await extractPdfText(file) : await file.text();
      if (!text.trim()) {
        setError("That file has no extractable text (a scanned PDF?). Paste the text instead.");
        return;
      }
      setContent(text);
      if (!title.trim()) setTitle(file.name.replace(/\.[^./\\]+$/, ""));
    } catch {
      setError("Couldn't read that file. Try a .pdf, .txt or .md — or paste the text below.");
    } finally {
      setReading(false);
    }
  }

  function onPick(e: React.ChangeEvent<HTMLInputElement>) {
    const f = e.target.files?.[0];
    if (f) handleFile(f);
    e.target.value = ""; // let the user re-pick the same file
  }

  function onDrop(e: React.DragEvent) {
    e.preventDefault();
    setDragging(false);
    const f = e.dataTransfer.files?.[0];
    if (f) handleFile(f);
  }

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
        setFileName("");
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : "failed to add document"))
      .finally(() => setSubmitting(false));
  }

  async function onDelete(docId: string) {
    const token = getToken();
    if (!token || busyId) return;
    setBusyId(docId);
    setError("");
    try {
      await deleteDocument(token, workspaceId, docId);
      setDocuments((prev) => prev.filter((d) => d.id !== docId));
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to delete document");
    } finally {
      setBusyId("");
    }
  }

  async function onRetry(docId: string) {
    const token = getToken();
    if (!token || busyId) return;
    setBusyId(docId);
    setError("");
    try {
      await reingestDocument(token, workspaceId, docId);
      setDocuments((prev) =>
        prev.map((d) => (d.id === docId ? { ...d, status: "pending", error: undefined } : d)),
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to retry ingestion");
    } finally {
      setBusyId("");
    }
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
          <div className="flex items-center gap-2">
            <Link
              href={`/workspaces/${workspaceId}/billing`}
              className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
            >
              ⚙️ Plan &amp; usage
            </Link>
            <Link
              href={`/workspaces/${workspaceId}/chat`}
              className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-700"
            >
              💬 Ask your documents
            </Link>
          </div>
        </div>
        <p className="mt-1 text-sm text-slate-500">
          Drop in a PDF or text file — we extract the text in your browser, then chunk, embed and
          store it for retrieval. Ingestion runs asynchronously.
        </p>

        <form
          onSubmit={onSubmit}
          className="mt-6 space-y-3 rounded-2xl border border-slate-200 bg-white p-6 shadow-sm"
        >
          <div
            role="button"
            tabIndex={0}
            onClick={() => fileRef.current?.click()}
            onKeyDown={(e) => (e.key === "Enter" || e.key === " ") && fileRef.current?.click()}
            onDragOver={(e) => {
              e.preventDefault();
              setDragging(true);
            }}
            onDragLeave={() => setDragging(false)}
            onDrop={onDrop}
            className={`flex cursor-pointer flex-col items-center justify-center rounded-xl border-2 border-dashed px-4 py-8 text-center transition ${
              dragging
                ? "border-indigo-400 bg-indigo-50"
                : "border-slate-300 hover:border-indigo-300 hover:bg-slate-50"
            }`}
          >
            <input
              ref={fileRef}
              type="file"
              accept=".pdf,.txt,.md,.markdown,.csv,.json,.log,text/*,application/pdf"
              className="hidden"
              onChange={onPick}
            />
            <div className="text-3xl">{reading ? "⏳" : "📎"}</div>
            <p className="mt-2 text-sm font-medium text-slate-700">
              {reading
                ? "Reading file…"
                : fileName
                  ? `Loaded “${fileName}” — review below, or drop another`
                  : "Drop a PDF or text file here, or click to choose"}
            </p>
            <p className="mt-1 text-xs text-slate-400">
              PDF, .txt, .md · extracted in your browser, nothing else is uploaded
            </p>
          </div>

          <input
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="Document title"
            className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200"
          />
          <textarea
            value={content}
            onChange={(e) => setContent(e.target.value)}
            placeholder="…or paste / edit the document text"
            rows={4}
            className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200"
          />
          <div className="flex items-center justify-between">
            <span className="text-xs text-slate-400">
              {content.trim() ? `${content.trim().length.toLocaleString()} characters` : ""}
            </span>
            <button
              type="submit"
              disabled={submitting || reading || !title.trim() || !content.trim()}
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
                className="flex items-center justify-between gap-3 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm"
              >
                <div className="flex min-w-0 items-center gap-3">
                  <div className="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl bg-slate-100 text-slate-500">
                    📄
                  </div>
                  <div className="min-w-0">
                    <h3 className="truncate font-semibold text-slate-900">{doc.title}</h3>
                    <p className="text-xs text-slate-400">
                      {doc.status === "ready"
                        ? `${doc.chunk_count} chunks embedded`
                        : doc.status === "pending"
                          ? "Ingesting…"
                          : "Ingestion failed"}
                    </p>
                    {doc.status === "failed" && doc.error && (
                      <p className="mt-0.5 truncate text-xs text-red-500" title={doc.error}>
                        {doc.error}
                      </p>
                    )}
                  </div>
                </div>
                <div className="flex flex-shrink-0 items-center gap-2">
                  <span
                    className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${STATUS[doc.status] ?? "bg-slate-100 text-slate-600"}`}
                  >
                    {doc.status}
                  </span>
                  {doc.status === "failed" && (
                    <button
                      onClick={() => onRetry(doc.id)}
                      disabled={busyId === doc.id}
                      className="rounded-lg border border-slate-300 px-2.5 py-1 text-xs font-medium text-slate-700 hover:bg-slate-100 disabled:opacity-50"
                    >
                      {busyId === doc.id ? "…" : "Retry"}
                    </button>
                  )}
                  <button
                    onClick={() => onDelete(doc.id)}
                    disabled={busyId === doc.id}
                    aria-label="Delete document"
                    className="rounded-lg px-2.5 py-1 text-xs font-medium text-red-600 hover:bg-red-50 disabled:opacity-50"
                  >
                    Delete
                  </button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </main>
    </div>
  );
}
