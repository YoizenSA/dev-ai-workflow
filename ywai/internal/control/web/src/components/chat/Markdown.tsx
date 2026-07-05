import { Fragment, type ReactNode } from "react";
import { tokenize } from "./syntaxHighlight";

// Lightweight, dependency-free markdown renderer for chat messages. Covers the
// formatting AI replies actually use: fenced code, inline code, bold, italic,
// links, headings and bullet/numbered lists. It builds React nodes (never
// dangerouslySetInnerHTML), so all text is escaped by React.

// Parse inline markup (**bold**, *italic*, `code`, [text](url)) into nodes.
function parseInline(text: string, keyPrefix: string): ReactNode[] {
  const nodes: ReactNode[] = [];
  // Order matters: code first so its contents aren't re-parsed.
  const pattern =
    /(`[^`]+`)|(\*\*[^*]+\*\*)|(\*[^*]+\*|_[^_]+_)|(\[[^\]]+\]\([^)]+\))/;
  let rest = text;
  let i = 0;
  while (rest.length > 0) {
    const m = rest.match(pattern);
    if (!m || m.index === undefined) {
      nodes.push(rest);
      break;
    }
    if (m.index > 0) nodes.push(rest.slice(0, m.index));
    const token = m[0];
    const key = `${keyPrefix}-${i++}`;
    if (token.startsWith("`")) {
      nodes.push(<code key={key}>{token.slice(1, -1)}</code>);
    } else if (token.startsWith("**")) {
      nodes.push(<strong key={key}>{token.slice(2, -2)}</strong>);
    } else if (token.startsWith("[")) {
      const linkMatch = token.match(/\[([^\]]+)\]\(([^)]+)\)/);
      if (linkMatch) {
        nodes.push(
          <a key={key} href={linkMatch[2]} target="_blank" rel="noreferrer">
            {linkMatch[1]}
          </a>,
        );
      } else {
        nodes.push(token);
      }
    } else {
      nodes.push(<em key={key}>{token.slice(1, -1)}</em>);
    }
    rest = rest.slice(m.index + token.length);
  }
  return nodes;
}

export default function Markdown({ content }: { content: string }) {
  const blocks: ReactNode[] = [];
  const lines = content.split("\n");
  let i = 0;
  let key = 0;

  while (i < lines.length) {
    const line = lines[i];

    // Fenced code block.
    if (line.trimStart().startsWith("```")) {
      // Extract language from the opening fence (e.g. ```tsx, ```python)
      const fenceMatch = line.trimStart().match(/^```(\w+)?/);
      const lang = fenceMatch?.[1] || "";
      const code: string[] = [];
      i++;
      while (i < lines.length && !lines[i].trimStart().startsWith("```")) {
        code.push(lines[i]);
        i++;
      }
      i++; // skip closing fence
      const codeText = code.join("\n");

      if (lang) {
        const tokens = tokenize(lang, codeText);
        blocks.push(
          <pre key={key++} className={`lang-${lang}`}>
            <code>
              {tokens.map((t, ti) =>
                t.className ? (
                  <span key={ti} className={t.className}>{t.text}</span>
                ) : (
                  <span key={ti}>{t.text}</span>
                )
              )}
            </code>
          </pre>,
        );
      } else {
        blocks.push(
          <pre key={key++}>
            <code>{codeText}</code>
          </pre>,
        );
      }
      continue;
    }

    // Heading.
    const heading = line.match(/^(#{1,3})\s+(.*)$/);
    if (heading) {
      const level = heading[1].length;
      const Tag = (`h${level + 2}` as "h3" | "h4" | "h5");
      blocks.push(<Tag key={key++}>{parseInline(heading[2], `h${key}`)}</Tag>);
      i++;
      continue;
    }

    // List (bullet or numbered) — consume consecutive items.
    if (/^\s*([-*]|\d+\.)\s+/.test(line)) {
      const ordered = /^\s*\d+\.\s+/.test(line);
      const items: ReactNode[] = [];
      while (i < lines.length && /^\s*([-*]|\d+\.)\s+/.test(lines[i])) {
        const item = lines[i].replace(/^\s*([-*]|\d+\.)\s+/, "");
        items.push(<li key={items.length}>{parseInline(item, `li${key}-${items.length}`)}</li>);
        i++;
      }
      blocks.push(
        ordered ? <ol key={key++}>{items}</ol> : <ul key={key++}>{items}</ul>,
      );
      continue;
    }

    // Blank line → skip.
    if (line.trim() === "") {
      i++;
      continue;
    }

    // Paragraph — gather consecutive non-blank, non-special lines.
    const para: string[] = [];
    while (
      i < lines.length &&
      lines[i].trim() !== "" &&
      !lines[i].trimStart().startsWith("```") &&
      !/^(#{1,3})\s+/.test(lines[i]) &&
      !/^\s*([-*]|\d+\.)\s+/.test(lines[i])
    ) {
      para.push(lines[i]);
      i++;
    }
    blocks.push(
      <p key={key++}>
        {para.map((l, idx) => (
          <Fragment key={idx}>
            {idx > 0 && <br />}
            {parseInline(l, `p${key}-${idx}`)}
          </Fragment>
        ))}
      </p>,
    );
  }

  return <>{blocks}</>;
}
