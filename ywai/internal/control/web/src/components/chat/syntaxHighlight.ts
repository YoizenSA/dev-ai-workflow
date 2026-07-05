// Minimal regex-based syntax tokenizer. Outputs {text, className}[] for CSS coloring.
// This is APPROXIMATE — good enough for readability, not an IDE.

export interface Token {
  text: string;
  className?: string;
}

type TokenRule = [RegExp, string | undefined]; // [pattern, className]

const LANG_RULES: Record<string, TokenRule[]> = {
  js: [
    [/\b(?:const|let|var|function|return|if|else|for|while|do|switch|case|break|continue|new|delete|typeof|instanceof|in|of|import|export|default|from|async|await|yield|class|extends|super|this|throw|try|catch|finally|true|false|null|undefined|void|with)\b/, "syn-kw"],
    [/\/\/.*$/, "syn-cmt"],
    [/\/\*[\s\S]*?\*\//, "syn-cmt"],
    [/(['"`])(?:\\.|(?!\1)[^\\])*\1/, "syn-str"],
    [/\b\d+(?:\.\d+)?(?:[eE][+-]?\d+)?\b/, "syn-num"],
    [/\b([A-Za-z_$][\w$]*)\s*\(/, "syn-fn"],
  ],
  ts: [
    [/\b(?:const|let|var|function|return|if|else|for|while|do|switch|case|break|continue|new|delete|typeof|instanceof|in|of|import|export|default|from|async|await|yield|class|extends|super|this|throw|try|catch|finally|true|false|null|undefined|void|with|interface|type|enum|implements|abstract|readonly|private|protected|public|static|as|satisfies|keyof|never|unknown|any|asserts|declare|namespace|module)\b/, "syn-kw"],
    [/\/\/.*$/, "syn-cmt"],
    [/\/\*[\s\S]*?\*\//, "syn-cmt"],
    [/(['"`])(?:\\.|(?!\1)[^\\])*\1/, "syn-str"],
    [/\b\d+(?:\.\d+)?(?:[eE][+-]?\d+)?\b/, "syn-num"],
    [/\b([A-Za-z_$][\w$]*)\s*[<(]/, "syn-fn"],
    [/\b([A-Za-z_$][\w$]*)\s*\(/, "syn-fn"],
  ],
  json: [
    [/"(?:\\.|[^"\\])*"/, "syn-str"],
    [/\b(?:true|false|null)\b/, "syn-kw"],
    [/\b-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?\b/, "syn-num"],
  ],
  go: [
    [/\b(?:func|return|if|else|for|range|switch|case|break|continue|default|var|const|type|struct|interface|map|chan|go|defer|select|import|package|fallthrough|goto|nil|true|false|make|new|append|len|cap|error|string|int|bool|byte|rune|float64|uint|int64|uint64|complex128)\b/, "syn-kw"],
    [/\/\/.*$/, "syn-cmt"],
    [/\/\*[\s\S]*?\*\//, "syn-cmt"],
    [/(['"`])(?:\\.|(?!\1)[^\\])*\1/, "syn-str"],
    [/\b\d+(?:\.\d+)?(?:[eE][+-]?\d+)?\b/, "syn-num"],
    [/\b([A-Za-z_]\w*)\s*\(/, "syn-fn"],
  ],
  html: [
    [/&[a-z]+;/, "syn-kw"],
    [/<\/?[a-z][\w-]*(?:\s[^>]*)?\/?>/, "syn-kw"],
    [/"(?:\\.|[^"\\])*"/, "syn-str"],
    [/'[^']*'/, "syn-str"],
    [/<!--[\s\S]*?-->/, "syn-cmt"],
  ],
  css: [
    [/@[\w-]+/, "syn-kw"],
    [/[.#][\w-]+/, "syn-fn"],
    [/\b([a-z-]+)\s*:/, "syn-kw"],
    [/#[\da-fA-F]{3,8}\b/, "syn-num"],
    [/"(?:\\.|[^"\\])*"/, "syn-str"],
    [/\/\*[\s\S]*?\*\//, "syn-cmt"],
    [/\b\d+(?:\.\d+)?(?:px|em|rem|%|vh|vw|s|ms)?\b/, "syn-num"],
  ],
  python: [
    [/\b(?:def|return|if|elif|else|for|while|break|continue|import|from|as|class|try|except|finally|raise|with|pass|yield|async|await|in|is|not|and|or|True|False|None|lambda|global|nonlocal|del|assert|print|range|len|type|int|str|list|dict|set|tuple|self)\b/, "syn-kw"],
    [/#.*$/, "syn-cmt"],
    [/'''[\s\S]*?'''/, "syn-str"],
    [/"""[\s\S]*?"""/, "syn-str"],
    [/(['"])(?:\\.|(?!\1)[^\\])*\1/, "syn-str"],
    [/\b\d+(?:\.\d+)?\b/, "syn-num"],
    [/\b([A-Za-z_]\w*)\s*\(/, "syn-fn"],
  ],
  bash: [
    [/\b(?:if|then|else|elif|fi|for|while|do|done|case|esac|in|function|return|exit|export|local|source|echo|cd|ls|cat|grep|sed|awk|rm|cp|mv|mkdir|touch|chmod|chown|sudo|apt|yum|npm|go|docker|kubectl|git|curl|wget|set|unset|read|shift|trap|exec|eval|let|select|until)\b/, "syn-kw"],
    [/#.*$/, "syn-cmt"],
    [/(['"`])(?:\\.|(?!\1)[^\\])*\1/, "syn-str"],
    [/\b\d+\b/, "syn-num"],
  ],
  yaml: [
    [/^---+$/, "syn-cmt"],
    [/^\.\.\.$/, "syn-cmt"],
    [/^[^#\s]\S*(?=: )|^[^#\s]\S*(?=:\s*\n)/, "syn-kw"],
    [/"[^"]*"/, "syn-str"],
    [/'[^']*'/, "syn-str"],
    [/#.*$/, "syn-cmt"],
    [/\b(?:true|false|yes|no|on|off|null)\b/, "syn-kw"],
    [/\b\d+(?:\.\d+)?\b/, "syn-num"],
  ],
};

const LANG_ALIASES: Record<string, string> = {
  javascript: "js",
  typescript: "ts",
  node: "js",
  jsx: "js",
  tsx: "ts",
  shell: "bash",
  sh: "bash",
  zsh: "bash",
  dockerfile: "bash",
  py: "python",
  yml: "yaml",
  json5: "json",
};

function resolveLang(lang: string): string {
  const key = lang.toLowerCase().trim();
  return LANG_ALIASES[key] ?? key;
}

export function tokenize(lang: string, code: string): Token[] {
  const rules = LANG_RULES[resolveLang(lang)];
  if (!rules) {
    // No rules → return raw text with no highlighting
    return [{ text: code }];
  }

  const tokens: Token[] = [];
  let remaining = code;

  outer: while (remaining.length > 0) {
    for (const [pattern, className] of rules) {
      const m = remaining.match(pattern);
      if (m && m.index === 0) {
        tokens.push({ text: m[0], className: className || undefined });
        remaining = remaining.slice(m[0].length);
        continue outer;
      }
    }
    // No rule matched at position 0 → emit one char as plain text
    // (Doing one char at a time is O(n) but code blocks are small)
    tokens.push({ text: remaining[0] });
    remaining = remaining.slice(1);
  }

  return tokens;
}
