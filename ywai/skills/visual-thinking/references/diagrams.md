# Canvas — blocks and HTML contract

Render `../assets/canvas-template.html`. Self-contained shell: yz-ui tokens, Mermaid 11 via ESM CDN, html2canvas for PNG, PDF via the browser print pipeline. Mermaid needs network; theme toggle, PDF and the rest of the page work offline (diagrams degrade to their source text).

## Fill these slots (exact ids)

- `#meta-topic` — the topic, in the user's words. Short.
- `#meta-date` — today, `YYYY-MM-DD`.
- `#meta-status` — one of `explorando` / `en decisión` / `decidido`.
- `#canvas` — the body. Replace the placeholder comment with 2–5 `<section>` blocks picked from below. Pick what the discussion needs; never one of each.

## Output rules

- Write to OS temp: `%TEMP%/visual-thinking-<timestamp>.html` (Windows) or `$TMPDIR/visual-thinking-<timestamp>.html`. Never in the repo.
- Open it (`start` / `open` / `xdg-open`), tell the user the absolute path.
- The file is a snapshot. A landed decision means a fresh file with a fresh timestamp, not an edit.
- Prose inside blocks: one sentence per field, no hedging. If a block needs a paragraph, redraw the diagram.
- Code in code-sketch blocks: escape `<`, `>`, `&`. The page has no build step and no highlighter — mono on surface is the styling, `<b>` marks the one line that matters.

## Diagram craft

- One diagram, one point. Max ~12 nodes.
- Name nodes with the domain terms from `AGENTS.md` and the conversation. Never invent entity names.
- Mermaid handles graph shapes (`flowchart`, `mindmap`, `sequence`). Hand-built `<div>`s (the `.duo` grid, cards) handle side-by-side and editorial visuals. Mix them.
- Colour only for meaning: red = pain today, green = the proposal / settled, amber = open. classDefs use token colours (`#f87171` danger, `#34d399` success, `#fbbf24` warning) with 8-digit hex fills (`#dc262629`, `#10b98129`, `#fdbd2726`). Never `rgba()` inside `classDef`: its commas break the parser.
- Keep diagram sources left-aligned inside `<pre class="diagram">`. For `mindmap`, indentation defines the hierarchy — keep it consistent.

## Block: idea map

The overview. Use when opening a topic: what the idea touches and how the parts relate.

```html
<section>
  <h2>Idea map</h2>
  <div class="card diagram-box">
    <pre class="diagram">
mindmap
  root((Auth redesign))
    Sessions
      Token rotation
      Sliding expiry
    Login
      Password
      Passkeys
    ---- open
      Migration path?
    </pre>
  </div>
  <p class="kv"><b>Open:</b> {{open questions, comma separated}}</p>
</section>
```

If `mindmap` fails to render (older caches), fall back to `flowchart TD` with the root at the top.

## Block: before / after

Current state vs proposed state. Use for a concrete proposed change to a flow, structure or process.

```html
<section>
  <h2>Before / After</h2>
  <div class="duo">
    <div class="side now">
      <h4>Hoy</h4>
      <div class="card diagram-box">
        <pre class="diagram">
flowchart LR
  A[Checkout] --> B[Mailer]
  B -. sync .-> C[Inventory]
  classDef pain fill:#dc262629,stroke:#f87171;
        </pre>
      </div>
      <p class="kv">{{what hurts, one sentence}}</p>
    </div>
    <div class="side next">
      <h4>Propuesto</h4>
      <div class="card diagram-box">
        <pre class="diagram">
flowchart LR
  A[Checkout] --> Q[Queue]
  Q --> C[Inventory]
  classDef good fill:#10b98129,stroke:#34d399;
        </pre>
      </div>
      <p class="kv">{{what changes, one sentence}}</p>
    </div>
  </div>
</section>
```

## Block: options

Alternatives in competition. Use when >=2 options are on the table and one must win.

```html
<section>
  <h2>Options</h2>
  <article class="card option rec">
    <div class="row"><span class="badge good">Recommended</span><span class="badge">option name</span></div>
    <p class="kv">{{one-sentence shape of the option}}</p>
    <ul>
      <li>{{trade-off}}</li>
      <li>{{trade-off}}</li>
    </ul>
  </article>
  <article class="card option">
    <div class="row"><span class="badge warn">Viable</span><span class="badge">option name</span></div>
    <!-- same fields -->
  </article>
  <article class="card option">
    <div class="row"><span class="badge bad">Rejected</span><span class="badge">option name</span></div>
    <p class="kv">{{the load-bearing reason it loses}}</p>
  </article>
  <p class="kv"><b>Why:</b> {{one sentence on what tips the recommendation}}</p>
</section>
```

## Block: decision tree

What is settled, what is open, what unlocks what. Use when decisions depend on each other. Pairs with the grilling skill: this is its design tree, drawn.

```html
<section>
  <h2>Decision tree</h2>
  <div class="card diagram-box">
    <pre class="diagram">
flowchart TD
  D1{"Auth model?"} -->|sessions| D2{"Token store?"}
  D1 -->|JWT| D3{"Refresh?"}
  D2 --> S1["Redis — settled"]
  classDef settled fill:#10b98129,stroke:#34d399;
  classDef openQ fill:#fdbd2726,stroke:#fbbf24,stroke-dasharray:5;
  class S1 settled
  class D1,D2,D3 openQ
    </pre>
  </div>
  <p class="kv"><b>Frontier:</b> {{the questions answerable now}} — <span class="badge open">open</span></p>
</section>
```

Settled nodes solid green, open nodes dashed amber rhombuses (`{"..."}`). The frontier line names what can be decided next without guessing.

## Block: sequence

Interaction over time. Use for API flows, event chains, and the classic "before: N round-trips; after: 1". Solid `->>` is a call, dashed `-->>` is a reply.

```html
<section>
  <h2>Sequence</h2>
  <div class="card diagram-box">
    <pre class="diagram">
sequenceDiagram
  participant C as Checkout
  participant Q as Queue
  participant I as Inventory
  C->>Q: OrderPaid
  Q-->>I: Reserve
  I-->>Q: Reserved
  Note over C,I: one round-trip, async
    </pre>
  </div>
  <p class="kv"><b>Point:</b> {{what the sequence proves, one sentence}}</p>
</section>
```

Before/after round-trips pair well as two sequence diagrams inside one `.duo` grid, `.side.now` vs `.side.next`.

## Block: code sketch

The shape of the change in code: an interface sketch, pseudo-code, the hot line before and after. Use when the discussion is about code shape, not flows. Sketches, never dumps: max ~12 lines per side, elide with `…`, `<b>` marks the line that matters.

```html
<section>
  <h2>Code sketch</h2>
  <div class="duo">
    <div class="side now">
      <h4>Hoy</h4>
      <div class="card diagram-box">
<pre class="code">order.CalcTotal()
order.ApplyDiscounts()
<b>order.NotifyWarehouse()</b>  // leaks
order.AuditLog()  // …</pre>
      </div>
    </div>
    <div class="side next">
      <h4>Propuesto</h4>
      <div class="card diagram-box">
<pre class="code">checkout.Submit(order)
  <i>// internals stay inside</i></pre>
      </div>
    </div>
  </div>
  <p class="kv"><b>Point:</b> {{one sentence on what the sketch shows}}</p>
</section>
```

The sequence and code blocks compose with before/after: a flow change often earns its keep only when the sequence or the sketch shows the difference.
