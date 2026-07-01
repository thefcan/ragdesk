"use client";

import { useEffect, useRef, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import ReactMarkdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";
import { chat, ApiError, type Source, type ChatTurn } from "@/lib/api";
import { getToken, clearToken } from "@/lib/auth";

type Message = {
  role: "user" | "assistant";
  content: string;
  sources?: Source[];
  error?: string;
};

const EXAMPLES = [
  "Summarise the key points",
  "What are the main risks?",
  "List any action items or deadlines",
];

// Scroll a cited source card into view and flash it, so a [1] click lands
// exactly on the passage it came from.
function focusSource(id: string) {
  const el = document.getElementById(id);
  if (!el) return;
  el.scrollIntoView({ behavior: "smooth", block: "center" });
  el.classList.add("ring-2", "ring-indigo-400");
  window.setTimeout(() => el.classList.remove("ring-2", "ring-indigo-400"), 1400);
}

export default function ChatPage() {
  const router = useRouter();
  const { id: workspaceId } = useParams<{ id: string }>();

  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [streaming, setStreaming] = useState(false);

  const threadRef = useRef<HTMLDivElement>(null);
  const formRef = useRef<HTMLFormElement>(null);
  const taRef = useRef<HTMLTextAreaElement>(null);

  // Stick to the bottom while new tokens stream in — unless the user has
  // scrolled up to re-read, in which case leave them where they are.
  useEffect(() => {
    const el = threadRef.current;
    if (!el) return;
    const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 140;
    if (nearBottom) el.scrollTop = el.scrollHeight;
  }, [messages]);

  function patchLast(patch: (m: Message) => Message) {
    setMessages((prev) => {
      if (prev.length === 0) return prev;
      const next = prev.slice();
      next[next.length - 1] = patch(next[next.length - 1]);
      return next;
    });
  }

  function autosize(ta: HTMLTextAreaElement) {
    ta.style.height = "auto";
    ta.style.height = `${Math.min(ta.scrollHeight, 160)}px`;
  }

  async function send(raw: string) {
    const token = getToken();
    const q = raw.trim();
    if (!token || !q || streaming) return;

    // Prior turns become follow-up context (skip empties and failed answers).
    const history: ChatTurn[] = messages
      .filter((m) => m.content.trim() && !m.error)
      .map((m) => ({ role: m.role, content: m.content }));

    setMessages((prev) => [
      ...prev,
      { role: "user", content: q },
      { role: "assistant", content: "" },
    ]);
    setInput("");
    if (taRef.current) taRef.current.style.height = "auto";
    setStreaming(true);

    try {
      await chat(
        token,
        workspaceId,
        q,
        {
          onSources: (s) => patchLast((m) => ({ ...m, sources: s })),
          onToken: (t) => patchLast((m) => ({ ...m, content: m.content + t })),
          onError: (msg) => patchLast((m) => ({ ...m, error: msg })),
        },
        history,
      );
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        clearToken();
        router.push("/login");
        return;
      }
      patchLast((m) => ({ ...m, error: err instanceof Error ? err.message : "chat failed" }));
    } finally {
      setStreaming(false);
    }
  }

  function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    void send(input);
  }

  function onKeyDown(e: React.KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      formRef.current?.requestSubmit();
    }
  }

  function newChat() {
    setMessages([]);
    setInput("");
    if (taRef.current) taRef.current.style.height = "auto";
    taRef.current?.focus();
  }

  function onLogout() {
    clearToken();
    router.push("/login");
  }

  return (
    <div className="flex h-[100dvh] flex-col bg-slate-50">
      <header className="shrink-0 border-b border-slate-200 bg-white">
        <div className="mx-auto flex max-w-3xl items-center justify-between px-6 py-4">
          <div className="flex items-center gap-3">
            <Link
              href={`/workspaces/${workspaceId}`}
              className="text-sm text-slate-400 hover:text-slate-700"
              title="Back to documents"
            >
              ←
            </Link>
            <Link href="/dashboard" className="text-lg font-bold tracking-tight">
              rag<span className="text-indigo-600">desk</span>
            </Link>
          </div>
          <div className="flex items-center gap-1">
            {messages.length > 0 && (
              <button
                onClick={newChat}
                className="rounded-lg px-3 py-1.5 text-sm font-medium text-slate-600 hover:bg-slate-100"
              >
                New chat
              </button>
            )}
            <button
              onClick={onLogout}
              className="rounded-lg px-3 py-1.5 text-sm font-medium text-slate-600 hover:bg-slate-100"
            >
              Sign out
            </button>
          </div>
        </div>
      </header>

      <div ref={threadRef} className="flex-1 overflow-y-auto">
        <div className="mx-auto max-w-3xl px-6 py-8">
          {messages.length === 0 ? (
            <EmptyState onPick={(q) => void send(q)} />
          ) : (
            <div className="space-y-6">
              {messages.map((m, i) =>
                m.role === "user" ? (
                  <UserBubble key={i} content={m.content} />
                ) : (
                  <AssistantBubble
                    key={i}
                    message={m}
                    index={i}
                    thinking={streaming && i === messages.length - 1}
                  />
                ),
              )}
            </div>
          )}
        </div>
      </div>

      <div className="shrink-0 border-t border-slate-200 bg-white">
        <form ref={formRef} onSubmit={onSubmit} className="mx-auto max-w-3xl px-6 py-4">
          <div className="flex items-end gap-2 rounded-2xl border border-slate-300 bg-white px-3 py-1.5 shadow-sm focus-within:border-indigo-500 focus-within:ring-2 focus-within:ring-indigo-200">
            <textarea
              ref={taRef}
              value={input}
              onChange={(e) => {
                setInput(e.target.value);
                autosize(e.target);
              }}
              onKeyDown={onKeyDown}
              rows={1}
              placeholder="Ask anything about your documents…"
              className="max-h-40 flex-1 resize-none bg-transparent py-1.5 leading-6 outline-none placeholder:text-slate-400"
            />
            <button
              type="submit"
              disabled={streaming || !input.trim()}
              aria-label="Send"
              className="mb-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-indigo-600 text-white transition hover:bg-indigo-700 disabled:opacity-40"
            >
              {streaming ? <Spinner /> : <SendIcon />}
            </button>
          </div>
          <p className="mt-1.5 text-center text-xs text-slate-400">
            Enter to send · Shift+Enter for a new line · answers are grounded in your documents
          </p>
        </form>
      </div>
    </div>
  );
}

function EmptyState({ onPick }: { onPick: (q: string) => void }) {
  return (
    <div className="mt-8 text-center">
      <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-indigo-100 text-xl font-bold text-indigo-700">
        rd
      </div>
      <h1 className="mt-4 text-2xl font-bold text-slate-900">Ask your documents</h1>
      <p className="mx-auto mt-1 max-w-md text-sm text-slate-500">
        Grounded answers with citations you can click to jump straight to the source.
      </p>
      <div className="mt-6 flex flex-wrap justify-center gap-2">
        {EXAMPLES.map((ex) => (
          <button
            key={ex}
            onClick={() => onPick(ex)}
            className="rounded-full border border-slate-200 bg-white px-4 py-2 text-sm text-slate-700 shadow-sm transition hover:border-indigo-300 hover:bg-indigo-50 hover:text-indigo-700"
          >
            {ex}
          </button>
        ))}
      </div>
    </div>
  );
}

function UserBubble({ content }: { content: string }) {
  return (
    <div className="flex justify-end">
      <p className="max-w-[80%] whitespace-pre-wrap rounded-2xl bg-indigo-600 px-4 py-2.5 text-white">
        {content}
      </p>
    </div>
  );
}

function AssistantBubble({
  message,
  index,
  thinking,
}: {
  message: Message;
  index: number;
  thinking: boolean;
}) {
  // Turn each [n] citation marker into a clickable anchor scoped to this turn.
  const withCitations = message.content.replace(
    /\[(\d+)\](?!\()/g,
    (_, n: string) => `[${n}](#cite-${index}-${n})`,
  );

  const components: Components = {
    a({ href, children }) {
      const cite = /^#cite-(\d+)-(\d+)$/.exec(href ?? "");
      if (cite) {
        const n = cite[2];
        return (
          <sup>
            <button
              type="button"
              onClick={() => focusSource(`src-${index}-${n}`)}
              className="mx-0.5 rounded bg-indigo-100 px-1 text-[0.7em] font-semibold text-indigo-700 hover:bg-indigo-200"
              title={`Jump to source ${n}`}
            >
              {n}
            </button>
          </sup>
        );
      }
      return (
        <a href={href} target="_blank" rel="noreferrer" className="text-indigo-600 underline">
          {children}
        </a>
      );
    },
  };

  return (
    <div className="flex gap-3">
      <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-xl bg-indigo-100 text-sm font-bold text-indigo-700">
        rd
      </div>
      <div className="min-w-0 flex-1 rounded-2xl border border-slate-200 bg-white px-4 py-3 shadow-sm">
        {message.content ? (
          <div className="prose prose-sm prose-slate max-w-none prose-headings:mt-3 prose-headings:mb-1 prose-p:my-2 prose-li:my-0.5 prose-pre:my-2">
            <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
              {withCitations}
            </ReactMarkdown>
          </div>
        ) : thinking ? (
          <ThinkingDots />
        ) : null}

        {message.error && (
          <p className="mt-2 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">{message.error}</p>
        )}

        {message.sources && message.sources.length > 0 && (
          <div className="mt-4 border-t border-slate-100 pt-3">
            <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-400">
              Sources
            </p>
            <ul className="space-y-2">
              {message.sources.map((s, i) => (
                <li
                  key={`${s.document_id}-${i}`}
                  id={`src-${index}-${i + 1}`}
                  className="scroll-mt-24 rounded-lg bg-slate-50 px-3 py-2 transition-shadow"
                >
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
  );
}

function ThinkingDots() {
  return (
    <div className="flex gap-1 py-1.5" aria-label="Thinking">
      <span className="h-2 w-2 animate-bounce rounded-full bg-slate-300 [animation-delay:-0.3s]" />
      <span className="h-2 w-2 animate-bounce rounded-full bg-slate-300 [animation-delay:-0.15s]" />
      <span className="h-2 w-2 animate-bounce rounded-full bg-slate-300" />
    </div>
  );
}

function SendIcon() {
  return (
    <svg viewBox="0 0 24 24" className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth="2">
      <path strokeLinecap="round" strokeLinejoin="round" d="M5 12h14M13 6l6 6-6 6" />
    </svg>
  );
}

function Spinner() {
  return (
    <svg viewBox="0 0 24 24" className="h-5 w-5 animate-spin" fill="none" stroke="currentColor" strokeWidth="2">
      <path strokeLinecap="round" d="M12 3a9 9 0 1 0 9 9" />
    </svg>
  );
}
