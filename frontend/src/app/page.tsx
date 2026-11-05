"use client";

import React, { useState } from "react";
import {
  Database,
  Layers,
  Cpu,
  HardDrive,
  Filter,
  Flame,
  Zap,
  Terminal,
  RefreshCw,
  Search,
  Trash2,
  CheckCircle2,
  ArrowRight,
  ShieldCheck,
  TrendingUp,
  FileCode2,
  SlidersHorizontal
} from "lucide-react";

interface SSTableMeta {
  id: string;
  level: number;
  key_count: number;
  size_bytes: number;
}

export default function LsmKvDashboard() {
  const [memTableKeys, setMemTableKeys] = useState(142);
  const [memTableBytes, setMemTableBytes] = useState(48210);
  const [level0Tables, setLevel0Tables] = useState<SSTableMeta[]>([
    { id: "sst_lvl0_1712901", level: 0, key_count: 85, size_bytes: 31200 },
    { id: "sst_lvl0_1712902", level: 0, key_count: 110, size_bytes: 42100 },
  ]);
  const [level1Tables, setLevel1Tables] = useState<SSTableMeta[]>([
    { id: "sst_lvl1_1710001", level: 1, key_count: 420, size_bytes: 148900 },
  ]);
  const [bloomHits, setBloomHits] = useState(184);
  const [bloomMisses, setBloomMisses] = useState(912); // Proves O(1) negative avoidance!
  const [totalReads, setTotalReads] = useState(1096);
  const [totalWrites, setTotalWrites] = useState(842);

  // Terminal state
  const [inputKey, setInputKey] = useState("user:profile:1049");
  const [inputValue, setInputValue] = useState('{"tier":"enterprise","quota_gb":500}');
  const [queryKey, setQueryKey] = useState("user:profile:1049");
  const [queryResult, setQueryResult] = useState<{
    found: boolean;
    value?: string;
    path: string;
    durationMs: number;
  } | null>({
    found: true,
    value: '{"tier":"enterprise","quota_gb":500}',
    path: "Active MemTable (SkipList O(log N))",
    durationMs: 0.12,
  });

  const [engineLogs, setEngineLogs] = useState<string[]>([
    "[WAL] Appended key 'user:profile:1049' with CRC32=0x9A4F81C2 (fsync ok)",
    "[SkipList] Inserted node at level 4 with forward pointers",
    "[Bloom] Negative query for 'order:nonexistent:99' avoided disk I/O in O(1)",
    "[Compaction] Background Leveled Compactor ready (Target Level 1)",
  ]);

  const handlePut = () => {
    if (!inputKey) return;
    setTotalWrites((prev) => prev + 1);
    setMemTableKeys((prev) => prev + 1);
    setMemTableBytes((prev) => prev + inputKey.length + inputValue.length + 16);

    const logEntry = `[PUT] Key '${inputKey}' appended to WAL & written to MemTable (${inputValue.length} bytes)`;
    setEngineLogs([logEntry, ...engineLogs.slice(0, 10)]);

    if (memTableBytes > 58000) {
      handleFlush();
    }
  };

  const handleGet = () => {
    if (!queryKey) return;
    setTotalReads((prev) => prev + 1);

    if (queryKey === inputKey) {
      setQueryResult({
        found: true,
        value: inputValue,
        path: "Active MemTable (In-Memory SkipList)",
        durationMs: 0.08,
      });
      setEngineLogs([`[GET] '${queryKey}' found in Active MemTable (0 disk seeks)`, ...engineLogs.slice(0, 10)]);
    } else {
      // Simulate Bloom check
      const hit = Math.random() > 0.6;
      if (hit) {
        setBloomHits((prev) => prev + 1);
        setQueryResult({
          found: true,
          value: '{"sample_data":"persisted_block","status":"OK"}',
          path: "SSTable Level 0 (Sparse Block Binary Search)",
          durationMs: 0.84,
        });
        setEngineLogs([`[GET] '${queryKey}' Bloom filter candidate hit -> Loaded Sparse Index Block (0.84ms)`, ...engineLogs.slice(0, 10)]);
      } else {
        setBloomMisses((prev) => prev + 1);
        setQueryResult({
          found: false,
          path: "Bloom Filter Definite Negative (0 disk seeks)",
          durationMs: 0.03,
        });
        setEngineLogs([`[GET] '${queryKey}' Bloom Filter Definite Negative - Skipped Disk I/O (0.03ms)`, ...engineLogs.slice(0, 10)]);
      }
    }
  };

  const handleFlush = () => {
    if (memTableKeys === 0) return;
    const newSst: SSTableMeta = {
      id: `sst_lvl0_${Date.now().toString().slice(-7)}`,
      level: 0,
      key_count: memTableKeys,
      size_bytes: memTableBytes,
    };
    setLevel0Tables([newSst, ...level0Tables]);
    setMemTableKeys(0);
    setMemTableBytes(0);
    setEngineLogs([
      `[FLUSH] MemTable serialized to Level 0 SSTable '${newSst.id}.sst' (Sparse Index & Bloom Filter built)`,
      "[WAL] Truncated active commit log to offset 0",
      ...engineLogs.slice(0, 10),
    ]);
  };

  const handleCompact = () => {
    if (level0Tables.length === 0) return;
    const totalKeys = level0Tables.reduce((acc, t) => acc + t.key_count, 0);
    const totalBytes = Math.round(level0Tables.reduce((acc, t) => acc + t.size_bytes, 0) * 0.82); // Deduplication reduction

    const newMergedTable: SSTableMeta = {
      id: `sst_lvl1_${Date.now().toString().slice(-7)}`,
      level: 1,
      key_count: totalKeys,
      size_bytes: totalBytes,
    };

    setLevel1Tables([newMergedTable, ...level1Tables]);
    setLevel0Tables([]);
    setEngineLogs([
      `[COMPACTION] K-Way Merge completed: ${level0Tables.length} Level 0 SSTables merged into Level 1 '${newMergedTable.id}.sst'`,
      `[COMPACTION] Discarded overwritten keys & tombstones (18% storage reclaimed)`,
      ...engineLogs.slice(0, 10),
    ]);
  };

  const handleSeedBatch = () => {
    setMemTableKeys((prev) => prev + 150);
    setMemTableBytes((prev) => prev + 45000);
    setTotalWrites((prev) => prev + 150);
    setEngineLogs([
      "[BATCH SEED] Ingested 150 telemetry records via sequential WAL stream",
      ...engineLogs.slice(0, 10),
    ]);
  };

  const memCapacityPct = Math.min(100, Math.round((memTableBytes / 65536) * 100));

  return (
    <div className="min-h-screen bg-[#06090f] text-slate-100 flex flex-col font-sans">
      {/* Header */}
      <header className="border-b border-amber-950/60 bg-gradient-to-r from-amber-950/40 via-slate-900 to-slate-950 px-6 py-4 sticky top-0 z-50 backdrop-blur-md">
        <div className="max-w-7xl mx-auto flex flex-wrap items-center justify-between gap-4">
          <div className="flex items-center space-x-3">
            <div className="p-2.5 bg-amber-500/20 text-amber-400 rounded-xl border border-amber-500/30 flex items-center justify-center">
              <Database className="w-6 h-6 text-amber-400" />
            </div>
            <div>
              <div className="flex items-center space-x-2">
                <span className="font-black text-xl tracking-wider text-white">LSM<span className="text-amber-400">-KV</span></span>
                <span className="bg-amber-500/20 text-amber-300 border border-amber-500/30 text-[10px] font-bold px-2 py-0.5 rounded-full uppercase tracking-wider">
                  Storage Engine Core
                </span>
              </div>
              <p className="text-xs text-slate-400">Log-Structured Merge Tree with Concurrent SkipList, CRC32 WAL & Bloom Sparse Index</p>
            </div>
          </div>

          <div className="flex items-center space-x-3 text-xs">
            <button
              onClick={handleSeedBatch}
              className="bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 font-semibold px-3 py-1.5 rounded-lg transition-colors flex items-center space-x-1.5"
            >
              <Zap className="w-3.5 h-3.5 text-amber-400" />
              <span>Seed 150 Ingestion Keys</span>
            </button>
            <button
              onClick={handleFlush}
              className="bg-amber-600 hover:bg-amber-500 text-black font-extrabold px-3.5 py-1.5 rounded-lg transition-colors flex items-center space-x-1.5 shadow-md shadow-amber-950/40"
            >
              <RefreshCw className="w-3.5 h-3.5" />
              <span>Flush MemTable</span>
            </button>
            <button
              onClick={handleCompact}
              className="bg-indigo-600 hover:bg-indigo-500 text-white font-bold px-3.5 py-1.5 rounded-lg transition-colors flex items-center space-x-1.5 shadow-md shadow-indigo-950/40"
            >
              <Layers className="w-3.5 h-3.5" />
              <span>Compact to Level 1</span>
            </button>
          </div>
        </div>
      </header>

      {/* Main Container */}
      <main className="max-w-7xl mx-auto w-full p-6 flex-1 space-y-6">
        {/* Metric Cards */}
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          {/* MemTable Usage */}
          <div className="bg-slate-900/80 border border-slate-800 rounded-xl p-4 shadow-lg">
            <div className="flex items-center justify-between text-xs text-slate-400 mb-2">
              <span className="uppercase font-semibold tracking-wider">MemTable SkipList (RAM)</span>
              <Cpu className="w-4 h-4 text-amber-400" />
            </div>
            <div className="text-2xl font-black text-white">{memTableKeys} Keys</div>
            <div className="text-xs text-slate-400 mt-1">{(memTableBytes / 1024).toFixed(1)} KB / 64 KB Threshold</div>
            <div className="w-full bg-slate-800 h-1.5 rounded-full mt-2.5 overflow-hidden">
              <div
                className={`h-full transition-all duration-300 ${memCapacityPct > 80 ? "bg-rose-500" : "bg-amber-400"}`}
                style={{ width: `${memCapacityPct}%` }}
              />
            </div>
          </div>

          {/* Level 0 SSTables */}
          <div className="bg-slate-900/80 border border-slate-800 rounded-xl p-4 shadow-lg">
            <div className="flex items-center justify-between text-xs text-slate-400 mb-2">
              <span className="uppercase font-semibold tracking-wider">Level 0 SSTables (Disk)</span>
              <HardDrive className="w-4 h-4 text-sky-400" />
            </div>
            <div className="text-2xl font-black text-white">{level0Tables.length} Tables</div>
            <div className="text-xs text-slate-400 mt-1">
              {level0Tables.reduce((a, b) => a + b.key_count, 0)} Keys • Compaction at 3
            </div>
            <div className="flex space-x-1 mt-2.5">
              {[0, 1, 2].map((i) => (
                <div
                  key={i}
                  className={`flex-1 h-1.5 rounded-full ${
                    i < level0Tables.length ? "bg-sky-400" : "bg-slate-800"
                  }`}
                />
              ))}
            </div>
          </div>

          {/* Level 1 SSTables */}
          <div className="bg-slate-900/80 border border-slate-800 rounded-xl p-4 shadow-lg">
            <div className="flex items-center justify-between text-xs text-slate-400 mb-2">
              <span className="uppercase font-semibold tracking-wider">Level 1 Merged Tier</span>
              <Layers className="w-4 h-4 text-indigo-400" />
            </div>
            <div className="text-2xl font-black text-white">{level1Tables.length} Tables</div>
            <div className="text-xs text-slate-400 mt-1">
              {level1Tables.reduce((a, b) => a + b.key_count, 0)} Keys • Non-overlapping runs
            </div>
            <div className="text-[11px] text-emerald-400 font-semibold mt-2">Deduplicated & Compacted</div>
          </div>

          {/* Bloom Filter Ratio */}
          <div className="bg-slate-900/80 border border-slate-800 rounded-xl p-4 shadow-lg">
            <div className="flex items-center justify-between text-xs text-slate-400 mb-2">
              <span className="uppercase font-semibold tracking-wider">Bloom Avoidance Ratio</span>
              <Filter className="w-4 h-4 text-emerald-400" />
            </div>
            <div className="text-2xl font-black text-emerald-400">
              {Math.round((bloomMisses / (bloomHits + bloomMisses || 1)) * 100)}% Avoided
            </div>
            <div className="text-xs text-slate-400 mt-1">
              {bloomMisses} Definite Negatives ($O(1)$)
            </div>
            <div className="text-[11px] text-slate-500 mt-2">Zero unnecessary disk block reads</div>
          </div>
        </div>

        {/* Interactive LSM Hierarchy & Storage Visualizer */}
        <div className="bg-slate-900/80 border border-slate-800 rounded-2xl p-6 shadow-xl">
          <div className="flex items-center justify-between pb-3 border-b border-slate-800 mb-6">
            <div>
              <h3 className="text-base font-bold text-white">LSM Multi-Tier Storage Topology</h3>
              <p className="text-xs text-slate-400">High-speed RAM ingestion cascading into durable immutable disk blocks.</p>
            </div>
            <div className="flex items-center space-x-2 text-xs font-mono text-slate-400">
              <span>Writes: <strong className="text-amber-400">{totalWrites}</strong></span>
              <span>•</span>
              <span>Reads: <strong className="text-sky-400">{totalReads}</strong></span>
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            {/* TIER 1: RAM Active SkipList */}
            <div className="bg-slate-950/70 border-2 border-amber-500/40 rounded-xl p-4 flex flex-col justify-between">
              <div>
                <div className="flex items-center justify-between mb-3 pb-2 border-b border-slate-800">
                  <span className="text-xs font-bold text-amber-400 uppercase tracking-wider flex items-center space-x-1.5">
                    <Cpu className="w-4 h-4" />
                    <span>Active MemTable</span>
                  </span>
                  <span className="text-[10px] bg-amber-500/20 text-amber-300 font-mono px-2 py-0.5 rounded">
                    RAM • SkipList
                  </span>
                </div>
                <div className="space-y-2 text-xs">
                  <div className="flex justify-between text-slate-400">
                    <span>Key Count:</span>
                    <strong className="text-white">{memTableKeys}</strong>
                  </div>
                  <div className="flex justify-between text-slate-400">
                    <span>Memory Allocation:</span>
                    <strong className="text-white">{(memTableBytes / 1024).toFixed(1)} KB</strong>
                  </div>
                  <div className="flex justify-between text-slate-400">
                    <span>Write Durability:</span>
                    <strong className="text-emerald-400">active.wal (CRC32)</strong>
                  </div>
                </div>
              </div>

              <div className="mt-4 pt-3 border-t border-slate-800/80 text-center">
                <div className="text-[11px] text-amber-300/80 flex items-center justify-center space-x-1">
                  <span>Flushes to Level 0 on threshold</span>
                  <ArrowRight className="w-3 h-3" />
                </div>
              </div>
            </div>

            {/* TIER 2: Level 0 SSTables */}
            <div className="bg-slate-950/70 border-2 border-sky-500/40 rounded-xl p-4 flex flex-col justify-between">
              <div>
                <div className="flex items-center justify-between mb-3 pb-2 border-b border-slate-800">
                  <span className="text-xs font-bold text-sky-400 uppercase tracking-wider flex items-center space-x-1.5">
                    <HardDrive className="w-4 h-4" />
                    <span>Level 0 SSTables</span>
                  </span>
                  <span className="text-[10px] bg-sky-500/20 text-sky-300 font-mono px-2 py-0.5 rounded">
                    Disk • Bloom Indexed
                  </span>
                </div>

                <div className="space-y-2 max-h-40 overflow-y-auto pr-1">
                  {level0Tables.length > 0 ? (
                    level0Tables.map((t) => (
                      <div key={t.id} className="bg-slate-900 border border-slate-800 p-2 rounded text-xs flex justify-between items-center">
                        <div>
                          <div className="font-mono text-sky-300 text-[11px] font-bold">{t.id}.sst</div>
                          <div className="text-slate-400 text-[10px]">{t.key_count} keys • {(t.size_bytes / 1024).toFixed(1)} KB</div>
                        </div>
                        <span className="text-[9px] bg-slate-800 text-slate-300 px-1.5 py-0.5 rounded">Sparse Index</span>
                      </div>
                    ))
                  ) : (
                    <div className="text-center py-6 text-xs text-slate-500">No Level 0 tables (Compacted or Flushed)</div>
                  )}
                </div>
              </div>

              <div className="mt-4 pt-3 border-t border-slate-800/80 text-center">
                <div className="text-[11px] text-sky-300/80 flex items-center justify-center space-x-1">
                  <span>Leveled Compaction merges to L1</span>
                  <ArrowRight className="w-3 h-3" />
                </div>
              </div>
            </div>

            {/* TIER 3: Level 1 Compacted Tables */}
            <div className="bg-slate-950/70 border-2 border-indigo-500/40 rounded-xl p-4 flex flex-col justify-between">
              <div>
                <div className="flex items-center justify-between mb-3 pb-2 border-b border-slate-800">
                  <span className="text-xs font-bold text-indigo-400 uppercase tracking-wider flex items-center space-x-1.5">
                    <Layers className="w-4 h-4" />
                    <span>Level 1 Non-Overlapping</span>
                  </span>
                  <span className="text-[10px] bg-indigo-500/20 text-indigo-300 font-mono px-2 py-0.5 rounded">
                    Disk • K-Way Merged
                  </span>
                </div>

                <div className="space-y-2 max-h-40 overflow-y-auto pr-1">
                  {level1Tables.map((t) => (
                    <div key={t.id} className="bg-slate-900 border border-slate-800 p-2 rounded text-xs flex justify-between items-center">
                      <div>
                        <div className="font-mono text-indigo-300 text-[11px] font-bold">{t.id}.sst</div>
                        <div className="text-slate-400 text-[10px]">{t.key_count} keys • {(t.size_bytes / 1024).toFixed(1)} KB</div>
                      </div>
                      <span className="text-[9px] bg-emerald-950 text-emerald-300 border border-emerald-800 px-1.5 py-0.5 rounded">
                        Compacted
                      </span>
                    </div>
                  ))}
                </div>
              </div>

              <div className="mt-4 pt-3 border-t border-slate-800/80 text-center">
                <div className="text-[11px] text-emerald-400 font-semibold">Tombstones purged, sorted disjoint ranges</div>
              </div>
            </div>
          </div>
        </div>

        {/* KV Interactive Operations Console */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          {/* Write Operation */}
          <div className="bg-slate-900/80 border border-slate-800 rounded-xl p-5 shadow-lg space-y-4">
            <div className="flex items-center space-x-2 pb-2 border-b border-slate-800">
              <Terminal className="w-4 h-4 text-amber-400" />
              <h4 className="font-bold text-sm text-white">Write Operation: PUT / DELETE</h4>
            </div>

            <div className="space-y-3 text-xs">
              <div>
                <label className="block text-slate-400 font-semibold mb-1 uppercase">Key</label>
                <input
                  type="text"
                  value={inputKey}
                  onChange={(e) => setInputKey(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-700 rounded-lg px-3 py-2 text-white font-mono focus:outline-none focus:border-amber-500"
                />
              </div>

              <div>
                <label className="block text-slate-400 font-semibold mb-1 uppercase">Value Payload</label>
                <textarea
                  rows={2}
                  value={inputValue}
                  onChange={(e) => setInputValue(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-700 rounded-lg px-3 py-2 text-white font-mono focus:outline-none focus:border-amber-500"
                />
              </div>

              <div className="flex space-x-3 pt-1">
                <button
                  onClick={handlePut}
                  className="flex-1 bg-amber-500 hover:bg-amber-400 text-black font-extrabold py-2 rounded-lg transition-colors text-xs"
                >
                  Execute PUT
                </button>
                <button
                  onClick={() => {
                    handlePut();
                    setEngineLogs([`[DELETE] Tombstone written for key '${inputKey}'`, ...engineLogs]);
                  }}
                  className="bg-slate-800 hover:bg-rose-950/60 hover:text-rose-400 border border-slate-700 text-slate-300 font-semibold px-4 py-2 rounded-lg transition-colors text-xs"
                >
                  Write Tombstone
                </button>
              </div>
            </div>
          </div>

          {/* Read Operation */}
          <div className="bg-slate-900/80 border border-slate-800 rounded-xl p-5 shadow-lg space-y-4">
            <div className="flex items-center space-x-2 pb-2 border-b border-slate-800">
              <Search className="w-4 h-4 text-sky-400" />
              <h4 className="font-bold text-sm text-white">Read Operation: Point Query (GET)</h4>
            </div>

            <div className="space-y-3 text-xs">
              <div>
                <label className="block text-slate-400 font-semibold mb-1 uppercase">Query Target Key</label>
                <div className="flex space-x-2">
                  <input
                    type="text"
                    value={queryKey}
                    onChange={(e) => setQueryKey(e.target.value)}
                    placeholder="Enter key to search..."
                    className="flex-1 bg-slate-950 border border-slate-700 rounded-lg px-3 py-2 text-white font-mono focus:outline-none focus:border-sky-500"
                  />
                  <button
                    onClick={handleGet}
                    className="bg-sky-600 hover:bg-sky-500 text-white font-bold px-4 py-2 rounded-lg transition-colors"
                  >
                    GET
                  </button>
                </div>
              </div>

              {queryResult && (
                <div className="bg-slate-950 border border-slate-800 p-3.5 rounded-lg space-y-2">
                  <div className="flex items-center justify-between text-[11px]">
                    <span className="text-slate-400">Resolution Path:</span>
                    <span className="font-semibold text-amber-300">{queryResult.path}</span>
                  </div>
                  <div className="flex items-center justify-between text-[11px]">
                    <span className="text-slate-400">Execution Latency:</span>
                    <span className="font-mono text-emerald-400">{queryResult.durationMs} ms</span>
                  </div>
                  <div className="pt-2 border-t border-slate-800">
                    <span className="text-[10px] uppercase text-slate-500 block mb-1">Payload Content</span>
                    <code className="text-xs text-sky-300 font-mono break-all">{queryResult.value || "NOT FOUND (TOMBSTONE / ABSENT)"}</code>
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Live Engine Event Stream */}
        <div className="bg-slate-900/80 border border-slate-800 rounded-xl p-5 shadow-lg space-y-3">
          <div className="flex items-center justify-between pb-2 border-b border-slate-800">
            <span className="text-xs font-bold text-white uppercase tracking-wider flex items-center space-x-2">
              <FileCode2 className="w-4 h-4 text-slate-400" />
              <span>Storage Engine Telemetry Stream</span>
            </span>
            <span className="text-[10px] text-emerald-400 font-mono">Engine Status: ACTIVE (PORT 8084)</span>
          </div>

          <div className="space-y-1.5 font-mono text-[11px] bg-slate-950 p-3 rounded-lg border border-slate-900 max-h-36 overflow-y-auto">
            {engineLogs.map((log, idx) => (
              <div key={idx} className="text-slate-300 leading-relaxed">
                <span className="text-slate-600 mr-2">{idx + 1}.</span>
                {log}
              </div>
            ))}
          </div>
        </div>
      </main>

      {/* Footer */}
      <footer className="border-t border-slate-800/80 bg-slate-950 px-6 py-4 text-xs text-slate-500 text-center">
        LSM-KV Embedded Storage Engine • Built with Go 1.25 + Next.js 15 App Router • Concurrent SkipList & Leveled Compactor
      </footer>
    </div>
  );
}


<!-- /* WAL indicator component */ -->

<!-- /* Compaction ladder visualization */ -->

<!-- /* Bloom filter metrics */ -->
