import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "LSM-KV | Log-Structured Merge Storage Engine",
  description: "High-throughput embedded key-value storage engine with MemTable SkipList, WAL, Bloom filter sparse index, and leveled compaction.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className="min-h-screen bg-[#070b12] text-slate-100 antialiased selection:bg-amber-500 selection:text-black">
        {children}
      </body>
    </html>
  );
}

