const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export type User = { id: string; email: string; created_at: string };
export type Workspace = {
  id: string;
  name: string;
  slug: string;
  owner_id: string;
  role?: string;
  created_at: string;
};

function headers(token?: string): HeadersInit {
  const h: Record<string, string> = { "Content-Type": "application/json" };
  if (token) h.Authorization = `Bearer ${token}`;
  return h;
}

async function handle<T>(res: Response): Promise<T> {
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error((data as { error?: string }).error ?? `request failed (${res.status})`);
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
