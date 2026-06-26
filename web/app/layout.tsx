import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "ragdesk — AI Knowledge SaaS",
  description: "Chat with your documents. Multi-tenant RAG with citations.",
};

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body className="min-h-screen bg-slate-50 text-slate-900 antialiased">{children}</body>
    </html>
  );
}
