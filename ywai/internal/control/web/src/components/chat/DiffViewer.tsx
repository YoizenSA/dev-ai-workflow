import { useState } from "react";

/* ── Types ─────────────────────────────────────────────────────── */

interface DiffLine {
  kind: "add" | "del" | "hunk" | "ctx" | "header";
  text: string;
}

interface DiffFile {
  oldName: string;
  newName: string;
  hunks: DiffLine[][];
  addCount: number;
  delCount: number;
}

/* ── Parser ────────────────────────────────────────────────────── */

function parseUnifiedDiff(raw: string): DiffFile[] {
  const lines = raw.split("\n");
  const files: DiffFile[] = [];
  let current: DiffFile | null = null;
  let currentHunk: DiffLine[] = [];

  function flushHunk() {
    if (current && currentHunk.length > 0) {
      current.hunks.push(currentHunk);
      currentHunk = [];
    }
  }

  for (const line of lines) {
    // File header: diff --git a/... b/...
    if (line.startsWith("diff --git")) {
      flushHunk();
      if (current) files.push(current);
      const m = line.match(/diff --git a\/(\S+) b\/(\S+)/);
      current = {
        oldName: m?.[1] ?? "",
        newName: m?.[2] ?? "",
        hunks: [],
        addCount: 0,
        delCount: 0,
      };
      continue;
    }

    // Per-file old/new header: --- a/... or +++ b/...
    if (line.startsWith("--- ")) {
      if (current) {
        const name = line.slice(4).trim().replace(/^a\//, "");
        if (!current.oldName) current.oldName = name;
      }
      continue;
    }
    if (line.startsWith("+++ ")) {
      if (current) {
        const name = line.slice(4).trim().replace(/^b\//, "");
        if (!current.newName) current.newName = name;
      }
      continue;
    }

    // Hunk header
    if (line.startsWith("@@")) {
      flushHunk();
      currentHunk.push({ kind: "hunk", text: line });
      continue;
    }

    // Added / removed / context
    if (line.startsWith("+")) {
      currentHunk.push({ kind: "add", text: line });
      if (current) current.addCount++;
      continue;
    }
    if (line.startsWith("-")) {
      currentHunk.push({ kind: "del", text: line });
      if (current) current.delCount++;
      continue;
    }

    // Context (space prefix) or anything else
    currentHunk.push({
      kind: line.startsWith(" ") ? "ctx" : "ctx",
      text: line,
    });
  }

  flushHunk();
  if (current) files.push(current);

  // Fallback: if no diff structure was found, treat whole input as context
  if (files.length === 0) {
    // Check if it looks like diff-ish content (has +/- lines)
    const hasAddDel = lines.some((l) => l.startsWith("+") || l.startsWith("-"));
    if (hasAddDel) {
      const fallback: DiffFile = {
        oldName: "",
        newName: "",
        hunks: [
          lines.map((l) => {
            if (l.startsWith("+")) return { kind: "add", text: l };
            if (l.startsWith("-")) return { kind: "del", text: l };
            return { kind: "ctx", text: l };
          }),
        ],
        addCount: lines.filter((l) => l.startsWith("+")).length,
        delCount: lines.filter((l) => l.startsWith("-")).length,
      };
      files.push(fallback);
    } else {
      // Not a diff at all
      return [];
    }
  }

  return files;
}

/* ── Component ─────────────────────────────────────────────────── */

function FileSection({ file, defaultOpen }: { file: DiffFile; defaultOpen: boolean }) {
  const [open, setOpen] = useState(defaultOpen);
  const stats =
    file.addCount > 0 || file.delCount > 0
      ? `+${file.addCount} / -${file.delCount} lines`
      : null;

  const filename = file.newName || file.oldName || "diff";

  return (
    <div className={`diff-file ${open ? "open" : ""}`}>
      <button className="diff-file-header" onClick={() => setOpen((o) => !o)}>
        <span className="diff-chevron">{open ? "▼" : "▶"}</span>
        <span className="diff-filename">{filename}</span>
        {stats && <span className="diff-stats">{stats}</span>}
      </button>
      {open && (
        <div className="diff-file-body">
          {file.hunks.map((hunk, hi) => (
            <div key={hi} className="diff-hunk">
              {hunk.map((line, li) => (
                <div key={li} className={`diff-line diff-line-${line.kind}`}>
                  <span className="diff-line-prefix">
                    {line.kind === "add" ? "+" : line.kind === "del" ? "−" : " "}
                  </span>
                  <span className="diff-line-text">{line.text.slice(line.kind === "hunk" ? 0 : 1)}</span>
                </div>
              ))}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

export default function DiffViewer({ content }: { content: string }) {
  if (!content || !content.trim()) return null;

  const files = parseUnifiedDiff(content);

  if (files.length === 0) {
    // Not a diff — render as raw pre
    return <pre className="diff-raw">{content}</pre>;
  }

  return (
    <div className="diff-viewer">
      {files.map((file, i) => (
        <FileSection key={i} file={file} defaultOpen={files.length <= 3} />
      ))}
    </div>
  );
}

/* ── Heuristic: does this string look like a diff? ─────────────── */

export function isDiffContent(text: string): boolean {
  if (!text || text.length < 20) return false;
  const lines = text.split("\n");
  // Check for typical diff markers
  const hasDashes = lines.some((l) => l.startsWith("--- a/"));
  const hasPluses = lines.some((l) => l.startsWith("+++ b/"));
  const hasHunk = lines.some((l) => l.startsWith("@@"));
  const hasDiffGit = lines.some((l) => l.startsWith("diff --git"));
  const hasAddDel = lines.filter((l) => l.startsWith("+") || l.startsWith("-")).length > 2;

  return hasDiffGit || (hasDashes && hasPluses && hasHunk) || (hasAddDel && (hasDashes || hasPluses || hasDiffGit));
}
