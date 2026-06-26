import Link from "next/link";

export default function Home() {
  return (
    <div className="flex min-h-screen flex-col">
      <header className="mx-auto flex w-full max-w-6xl items-center justify-between px-6 py-5">
        <span className="text-xl font-bold tracking-tight">
          rag<span className="text-indigo-600">desk</span>
        </span>
        <nav className="flex items-center gap-3 text-sm">
          <Link href="/login" className="rounded-lg px-4 py-2 font-medium text-slate-700 hover:bg-slate-100">
            Sign in
          </Link>
          <Link href="/register" className="rounded-lg bg-indigo-600 px-4 py-2 font-medium text-white hover:bg-indigo-700">
            Get started
          </Link>
        </nav>
      </header>

      <main className="mx-auto flex w-full max-w-6xl flex-1 flex-col items-center justify-center px-6 text-center">
        <span className="mb-5 inline-flex items-center gap-2 rounded-full border border-indigo-200 bg-indigo-50 px-4 py-1.5 text-sm font-medium text-indigo-700">
          ✨ Retrieval-Augmented Generation, as a product
        </span>
        <h1 className="max-w-3xl text-5xl font-extrabold tracking-tight text-slate-900 sm:text-6xl">
          Chat with your documents.
        </h1>
        <p className="mt-6 max-w-2xl text-lg text-slate-600">
          ragdesk gives every team a private, multi-tenant workspace where an AI assistant
          answers <span className="font-semibold text-slate-800">only from your documents</span> — with citations.
        </p>
        <div className="mt-9 flex items-center gap-4">
          <Link href="/register" className="rounded-xl bg-indigo-600 px-6 py-3 font-semibold text-white shadow-sm hover:bg-indigo-700">
            Create your workspace
          </Link>
          <Link href="/login" className="rounded-xl border border-slate-300 bg-white px-6 py-3 font-semibold text-slate-700 hover:bg-slate-50">
            Sign in
          </Link>
        </div>

        <div className="mt-20 grid w-full max-w-4xl gap-6 sm:grid-cols-3">
          {[
            { t: "Multi-tenant", d: "Isolated workspaces with roles and members." },
            { t: "Grounded answers", d: "Every response cites the source documents." },
            { t: "Runs on $0", d: "Local LLMs, open-source stack, no lock-in." },
          ].map((f) => (
            <div key={f.t} className="rounded-2xl border border-slate-200 bg-white p-6 text-left shadow-sm">
              <h3 className="font-semibold text-slate-900">{f.t}</h3>
              <p className="mt-2 text-sm text-slate-600">{f.d}</p>
            </div>
          ))}
        </div>
      </main>

      <footer className="mx-auto w-full max-w-6xl px-6 py-8 text-center text-sm text-slate-400">
        ragdesk — Go · Python · Next.js · Postgres/pgvector
      </footer>
    </div>
  );
}
