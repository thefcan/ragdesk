"use client";

import { useState } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { chat, ApiError, type Source } from "@/lib/api";
import { getToken, clearToken } from "@/lib/auth";

export default function ChatPage() {
  const router = useRouter();
  const { id: workspaceId } = useParams<{ id: string }>();

  const [question, setQuestion] = useState("");
  const [asked, setAsked] = useState("");
  const [answer, setAnswer] = useState("");
  const [sources, setSources] = useState<Source[]>([]);
  const [streaming, setStreaming] = useState(false);
  const [error, setError] = useState("");

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const token = getToken();
    if (!token || !question.trim() || streaming) return;
    const q = question.trim();
    setAsked(q);
    setQuestion("");
    setAnswer("");
    setSources([]);
    setError("");
    setStreaming(true);
    try {
      await chat(token, workspaceId, q, {
        onSources: setSources,
        onToken: (t) => setAnswer((prev) => prev + t),
        onError: (m) => setError(m),
      });
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        clearToken();
        router.push("/login");
        return;
      }
      setError(err instanceof Error ? err.message : "chat failed");
    } finally {
      setStreaming(false);
    }
  }

  function onLogout() {
    clearToken();
    router.push("/login");
  }

  return (
    <div className="min-h-screen">
      <header className="border-b border-slate-200 bg-white">
        <div className="mx-auto flex max-w-3xl items-center justify-between px-6 py-4">
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

      <main className="mx-auto max-w-3xl px-6 py-10">
        <Link
          href={`/workspaces/${workspaceId}`}
          className="text-sm text-slate-500 hover:text-slate-700"
        >
          ← Documents
        </Link>
        <h1 className="mt-2 text-2xl font-bold text-slate-900">Ask your documents</h1>
        <p className="mt-1 text-sm text-slate-500">
          Answers are grounded in this workspace&apos;s documents, with citations.
        </p>

        <form onSubmit={onSubmit} className="mt-6 flex gap-2">
          <input
            value={question}
            onChange={(e) => setQuestion(e.target.value)}
            placeholder="e.g. What is the home-office stipend?"
            className="flex-1 rounded-xl border border-slate-300 px-4 py-3 outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200"
          />
          <button
            type="submit"
            disabled={streaming}
            className="rounded-xl bg-indigo-600 px-6 py-3 font-semibold text-white hover:bg-indigo-700 disabled:opacity-60"
          >
            {streaming ? "Thinking…" : "Ask"}
          </button>
        </form>

        {error && <p className="mt-6 rounded-lg bg-red-50 px-4 py-2 text-sm text-red-700">{error}</p>}

        {asked && (
          <div className="mt-8 space-y-4">
            <div className="flex justify-end">
              <p className="max-w-[80%] rounded-2xl bg-indigo-600 px-4 py-2.5 text-white">{asked}</p>
            </div>

            <div className="flex gap-3">
              <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-xl bg-indigo-100 text-sm font-bold text-indigo-700">
                rd
              </div>
              <div className="min-w-0 flex-1 rounded-2xl border border-slate-200 bg-white px-4 py-3 shadow-sm">
                <p className="whitespace-pre-wrap text-slate-800">
                  {answer || (streaming ? "…" : "")}
                </p>

                {sources.length > 0 && (
                  <div className="mt-4 border-t border-slate-100 pt-3">
                    <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-400">
                      Sources
                    </p>
                    <ul className="space-y-2">
                      {sources.map((s, i) => (
                        <li key={`${s.document_id}-${i}`} className="rounded-lg bg-slate-50 px-3 py-2">
                          <p className="text-sm font-medium text-slate-800">
                            <span className="text-indigo-600">[{i + 1}]</span> {s.title}
                          </p>
                          <p className="mt-0.5 line-clamp-2 text-xs text-slate-500">{s.snippet}</p>
                        </li>
                      ))}
                    </ul>
                  </div>
                )}
              </div>
            </div>
          </div>
        )}
      </main>
    </div>
  );
}
