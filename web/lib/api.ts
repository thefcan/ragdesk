const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export type User = { id: string; email: string; created_at: string };
export type Workspace = {
  id: string;
  name: string;
  slug: string;
  owner_id: string;
  role?: string;
  plan?: string;
  subscription_status?: string;
  created_at: string;
};

function headers(token?: string): HeadersInit {
  const h: Record<string, string> = { "Content-Type": "application/json" };
  if (token) h.Authorization = `Bearer ${token}`;
  return h;
}

export class ApiError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

async function handle<T>(res: Response): Promise<T> {
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new ApiError((data as { error?: string }).error ?? `request failed (${res.status})`, res.status);
  }
  return data as T;
}

export function register(email: string, password: string, workspace?: string) {
  return fetch(`${API_URL}/auth/register`, {
    method: "POST",
    headers: headers(),
    body: JSON.stringify({ email, password, workspace }),
  }).then(handle<{ token: string; user: User; workspace: Workspace }>);
}

export function login(email: string, password: string) {
  return fetch(`${API_URL}/auth/login`, {
    method: "POST",
    headers: headers(),
    body: JSON.stringify({ email, password }),
  }).then(handle<{ token: string; user: User }>);
}

export function listWorkspaces(token: string) {
  return fetch(`${API_URL}/workspaces`, { headers: headers(token) }).then(
    handle<{ workspaces: Workspace[] }>,
  );
}

export function createWorkspace(token: string, name: string) {
  return fetch(`${API_URL}/workspaces`, {
    method: "POST",
    headers: headers(token),
    body: JSON.stringify({ name }),
  }).then(handle<Workspace>);
}

export type Document = {
  id: string;
  workspace_id: string;
  title: string;
  status: string;
  chunk_count: number;
  error?: string;
  created_at: string;
};

export function listDocuments(token: string, workspaceId: string) {
  return fetch(`${API_URL}/workspaces/${workspaceId}/documents`, {
    headers: headers(token),
  }).then(handle<{ documents: Document[] }>);
}

export function createDocument(token: string, workspaceId: string, title: string, content: string) {
  return fetch(`${API_URL}/workspaces/${workspaceId}/documents`, {
    method: "POST",
    headers: headers(token),
    body: JSON.stringify({ title, content }),
  }).then(handle<Document>);
}

export function reingestDocument(token: string, workspaceId: string, docId: string) {
  return fetch(`${API_URL}/workspaces/${workspaceId}/documents/${docId}/reingest`, {
    method: "POST",
    headers: headers(token),
  }).then(handle<{ status: string }>);
}

export function deleteDocument(token: string, workspaceId: string, docId: string) {
  return fetch(`${API_URL}/workspaces/${workspaceId}/documents/${docId}`, {
    method: "DELETE",
    headers: headers(token),
  }).then(handle<Record<string, never>>);
}

export type Plan = {
  id: string;
  name: string;
  price_cents: number;
  max_documents: number;
  max_chat_per_month: number;
};

export type Billing = {
  plan: string;
  status: string;
  role: string;
  period: string;
  usage: { documents: number; chat_messages: number };
  limits: { documents: number; chat_messages: number };
  plans: Plan[];
  billing_enabled: boolean;
};

export function getBilling(token: string, workspaceId: string) {
  return fetch(`${API_URL}/workspaces/${workspaceId}/billing`, {
    headers: headers(token),
  }).then(handle<Billing>);
}

export function checkout(token: string, workspaceId: string, plan: string) {
  return fetch(`${API_URL}/workspaces/${workspaceId}/billing/checkout`, {
    method: "POST",
    headers: headers(token),
    body: JSON.stringify({ plan }),
  }).then(handle<{ url: string; mode: string }>);
}

export function devConfirm(token: string, workspaceId: string, plan: string) {
  return fetch(`${API_URL}/workspaces/${workspaceId}/billing/dev-confirm`, {
    method: "POST",
    headers: headers(token),
    body: JSON.stringify({ plan }),
  }).then(handle<{ plan: string; status: string }>);
}

export function portal(token: string, workspaceId: string) {
  return fetch(`${API_URL}/workspaces/${workspaceId}/billing/portal`, {
    method: "POST",
    headers: headers(token),
  }).then(handle<{ url: string }>);
}

export function devCancel(token: string, workspaceId: string) {
  return fetch(`${API_URL}/workspaces/${workspaceId}/billing/dev-cancel`, {
    method: "POST",
    headers: headers(token),
  }).then(handle<{ plan: string; status: string }>);
}

export type Source = { document_id: string; title: string; snippet: string };

type ChatHandlers = {
  onSources?: (sources: Source[]) => void;
  onToken?: (token: string) => void;
  onError?: (message: string) => void;
};

export async function chat(
  token: string,
  workspaceId: string,
  question: string,
  handlers: ChatHandlers,
): Promise<void> {
  const res = await fetch(`${API_URL}/workspaces/${workspaceId}/chat`, {
    method: "POST",
    headers: headers(token),
    body: JSON.stringify({ question }),
  });
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new ApiError((data as { error?: string }).error ?? `chat failed (${res.status})`, res.status);
  }
  const reader = res.body?.getReader();
  if (!reader) return;

  const decoder = new TextDecoder();
  let buffer = "";
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    const lines = buffer.split("\n");
    buffer = lines.pop() ?? "";
    for (const line of lines) {
      if (!line.trim()) continue;
      const event = JSON.parse(line) as {
        type: string;
        sources?: Source[];
        content?: string;
      };
      if (event.type === "sources") handlers.onSources?.(event.sources ?? []);
      else if (event.type === "token") handlers.onToken?.(event.content ?? "");
      else if (event.type === "error") handlers.onError?.(event.content ?? "the assistant is unavailable");
    }
  }
}
