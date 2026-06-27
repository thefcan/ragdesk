"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { listWorkspaces, createWorkspace, ApiError, type Workspace } from "@/lib/api";
import { getToken, clearToken } from "@/lib/auth";

export default function DashboardPage() {
  const router = useRouter();
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [name, setName] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    const token = getToken();
    if (!token) {
      router.push("/login");
      return;
    }
    let active = true;
    listWorkspaces(token)
      .then((data) => {
        if (active) setWorkspaces(data.workspaces ?? []);
      })
      .catch((err: unknown) => {
        if (err instanceof ApiError && err.status === 401) {
          clearToken();
          router.push("/login");
          return;
        }
        if (active) setError(err instanceof Error ? err.message : "failed to load workspaces");
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [router]);

  async function onCreate(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const token = getToken();
    if (!token || !name.trim()) return;
    setCreating(true);
    setError("");
    try {
      const ws = await createWorkspace(token, name.trim());
      setWorkspaces((prev) => [...prev, ws]);
      setName("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to create workspace");
    } finally {
      setCreating(false);
    }
  }

  function onLogout() {
    clearToken();
    router.push("/login");
  }

  const palette = [
    "bg-indigo-500",
    "bg-violet-500",
    "bg-sky-500",
    "bg-emerald-500",
    "bg-amber-500",
    "bg-rose-500",
  ];

  return (
    <div className="min-h-screen">
      <header className="border-b border-slate-200 bg-white">
        <div className="mx-auto flex max-w-5xl items-center justify-between px-6 py-4">
          <span className="text-lg font-bold tracking-tight">
            rag<span className="text-indigo-600">desk</span>
          </span>
          <button
            onClick={onLogout}
            className="rounded-lg px-3 py-1.5 text-sm font-medium text-slate-600 hover:bg-slate-100"
          >
            Sign out
          </button>
        </div>
      </header>

      <main className="mx-auto max-w-5xl px-6 py-10">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <h1 className="text-2xl font-bold text-slate-900">Workspaces</h1>
            <p className="mt-1 text-sm text-slate-500">
              Each workspace is an isolated tenant with its own documents and members.
            </p>
          </div>
          <form onSubmit={onCreate} className="flex gap-2">
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="New workspace name"
              className="rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200"
            />
            <button
              type="submit"
              disabled={creating}
              className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-700 disabled:opacity-60"
            >
              {creating ? "Creating…" : "Create"}
            </button>
          </form>
        </div>

        {error && <p className="mt-6 rounded-lg bg-red-50 px-4 py-2 text-sm text-red-700">{error}</p>}

        {loading ? (
          <p className="mt-10 text-slate-500">Loading…</p>
        ) : (
          <div className="mt-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {workspaces.map((ws) => (
              <div
                key={ws.id}
                className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm transition hover:shadow-md"
              >
                <div className="flex items-center gap-3">
                  <div
                    className={`flex h-10 w-10 items-center justify-center rounded-xl text-lg font-bold text-white ${palette[ws.name.charCodeAt(0) % palette.length]}`}
                  >
                    {ws.name.charAt(0).toUpperCase()}
                  </div>
                  <div className="min-w-0">
                    <h3 className="truncate font-semibold text-slate-900">{ws.name}</h3>
                    <p className="truncate text-xs text-slate-400">/{ws.slug}</p>
                  </div>
                </div>
                <div className="mt-4 flex items-center justify-between">
                  {ws.role && (
                    <span className="rounded-full bg-indigo-50 px-2.5 py-0.5 text-xs font-medium text-indigo-700">
                      {ws.role}
                    </span>
                  )}
                  <span className="text-xs text-slate-400">
                    {new Date(ws.created_at).toLocaleDateString()}
                  </span>
                </div>
              </div>
            ))}
          </div>
        )}
      </main>
    </div>
  );
}
