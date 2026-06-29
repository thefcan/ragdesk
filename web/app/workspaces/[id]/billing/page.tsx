"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { getBilling, checkout, devConfirm, ApiError, type Billing } from "@/lib/api";
import { getToken, clearToken } from "@/lib/auth";

function UsageBar({ label, used, limit }: { label: string; used: number; limit: number }) {
  const pct = limit > 0 ? Math.min(100, Math.round((used / limit) * 100)) : 0;
  const near = pct >= 90;
  return (
    <div>
      <div className="flex items-center justify-between text-sm">
        <span className="font-medium text-slate-700">{label}</span>
        <span className="text-slate-500">
          {used.toLocaleString()} / {limit.toLocaleString()}
        </span>
      </div>
      <div className="mt-1.5 h-2.5 w-full overflow-hidden rounded-full bg-slate-100">
        <div
          className={`h-full rounded-full ${near ? "bg-red-500" : "bg-indigo-500"}`}
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  );
}

export default function BillingPage() {
  const router = useRouter();
  const { id: workspaceId } = useParams<{ id: string }>();

  const [billing, setBilling] = useState<Billing | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);

  const load = useCallback(() => {
    const token = getToken();
    if (!token) {
      router.push("/login");
      return;
    }
    getBilling(token, workspaceId)
      .then(setBilling)
      .catch((err: unknown) => {
        if (err instanceof ApiError && err.status === 401) {
          clearToken();
          router.push("/login");
          return;
        }
        setError(err instanceof Error ? err.message : "failed to load billing");
      })
      .finally(() => setLoading(false));
  }, [router, workspaceId]);

  useEffect(() => {
    const token = getToken();
    if (!token) {
      router.push("/login");
      return;
    }
    const params = new URLSearchParams(window.location.search);
    const flow = params.get("checkout");
    const plan = params.get("plan") ?? "pro";
    if (flow) window.history.replaceState({}, "", `/workspaces/${workspaceId}/billing`);

    if (flow === "dev") {
      // $0 mode: dev-confirm stands in for the Stripe webhook that would
      // otherwise fulfil the subscription after a hosted checkout.
      devConfirm(token, workspaceId, plan)
        .then(() => setNotice("Subscription activated."))
        .catch((err: unknown) => setError(err instanceof Error ? err.message : "activation failed"))
        .finally(load);
      return;
    }
    if (flow === "success") Promise.resolve().then(() => setNotice("Subscription activated."));
    else if (flow === "cancel") Promise.resolve().then(() => setNotice("Checkout canceled."));
    load();
  }, [load, router, workspaceId]);

  function onUpgrade(plan: string) {
    const token = getToken();
    if (!token) return;
    setBusy(true);
    setError("");
    checkout(token, workspaceId, plan)
      .then(({ url }) => {
        window.location.href = url;
      })
      .catch((err: unknown) => {
        setError(err instanceof Error ? err.message : "checkout failed");
        setBusy(false);
      });
  }

  function onLogout() {
    clearToken();
    router.push("/login");
  }

  const isOwner = billing?.role === "owner";

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
        <Link
          href={`/workspaces/${workspaceId}`}
          className="text-sm text-slate-500 hover:text-slate-700"
        >
          ← Documents
        </Link>
        <h1 className="mt-2 text-2xl font-bold text-slate-900">Plan &amp; usage</h1>
        <p className="mt-1 text-sm text-slate-500">
          Limits are enforced per workspace and usage resets monthly
          {billing ? ` (period ${billing.period}).` : "."}
        </p>

        {notice && (
          <p className="mt-6 rounded-lg bg-emerald-50 px-4 py-2 text-sm text-emerald-700">{notice}</p>
        )}
        {error && <p className="mt-6 rounded-lg bg-red-50 px-4 py-2 text-sm text-red-700">{error}</p>}

        {loading || !billing ? (
          <p className="mt-10 text-slate-500">Loading…</p>
        ) : (
          <>
            <section className="mt-6 rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-xs uppercase tracking-wide text-slate-400">Current plan</p>
                  <p className="mt-0.5 text-xl font-bold capitalize text-slate-900">{billing.plan}</p>
                </div>
                <span
                  className={`rounded-full px-3 py-1 text-xs font-medium ${
                    billing.status === "active"
                      ? "bg-emerald-50 text-emerald-700"
                      : "bg-amber-50 text-amber-700"
                  }`}
                >
                  {billing.status}
                </span>
              </div>
              <div className="mt-6 space-y-4">
                <UsageBar
                  label="Documents"
                  used={billing.usage.documents}
                  limit={billing.limits.documents}
                />
                <UsageBar
                  label="Chat messages this month"
                  used={billing.usage.chat_messages}
                  limit={billing.limits.chat_messages}
                />
              </div>
            </section>

            <div className="mt-8 grid gap-4 sm:grid-cols-2">
              {billing.plans.map((p) => {
                const current = p.id === billing.plan;
                return (
                  <div
                    key={p.id}
                    className={`rounded-2xl border p-6 shadow-sm ${
                      current ? "border-indigo-500 bg-indigo-50/40" : "border-slate-200 bg-white"
                    }`}
                  >
                    <div className="flex items-center justify-between">
                      <h3 className="text-lg font-bold text-slate-900">{p.name}</h3>
                      {current && (
                        <span className="rounded-full bg-indigo-600 px-2.5 py-0.5 text-xs font-semibold text-white">
                          Current
                        </span>
                      )}
                    </div>
                    <p className="mt-1 text-2xl font-bold text-slate-900">
                      {p.price_cents === 0 ? "Free" : `$${(p.price_cents / 100).toFixed(0)}`}
                      {p.price_cents > 0 && (
                        <span className="text-sm font-normal text-slate-400">/mo</span>
                      )}
                    </p>
                    <ul className="mt-4 space-y-1.5 text-sm text-slate-600">
                      <li>✓ {p.max_documents.toLocaleString()} documents</li>
                      <li>✓ {p.max_chat_per_month.toLocaleString()} chat messages / month</li>
                    </ul>
                    {!current && p.id !== "free" && (
                      <button
                        onClick={() => onUpgrade(p.id)}
                        disabled={busy || !isOwner}
                        className="mt-5 w-full rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-700 disabled:opacity-60"
                      >
                        {busy ? "Redirecting…" : `Upgrade to ${p.name}`}
                      </button>
                    )}
                  </div>
                );
              })}
            </div>

            {!isOwner && (
              <p className="mt-4 text-xs text-slate-400">
                Only the workspace owner can change the plan.
              </p>
            )}
            {!billing.billing_enabled && (
              <p className="mt-2 text-xs text-slate-400">
                Dev billing mode — upgrades are simulated (no charge). Set Stripe keys to enable
                hosted checkout.
              </p>
            )}
          </>
        )}
      </main>
    </div>
  );
}
