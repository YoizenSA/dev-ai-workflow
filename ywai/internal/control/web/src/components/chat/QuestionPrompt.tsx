import { useState } from "react";
import { HelpCircle, Send } from "lucide-react";

export interface QuestionInfo {
  question: string;
  header?: string;
  options: { label: string; description?: string }[];
  multiple?: boolean;
  custom?: boolean;
}

export interface QuestionRequest {
  id: string;
  sessionID: string;
  questions: QuestionInfo[];
}

// Renders a pending agent question and collects the user's answer. OpenCode
// blocks the turn until this is answered (or skipped).
export default function QuestionPrompt({
  request,
  onReply,
  onReject,
}: {
  request: QuestionRequest;
  onReply: (answers: string[][]) => void;
  onReject: () => void;
}) {
  const [selected, setSelected] = useState<string[][]>(
    request.questions.map(() => []),
  );
  const [customText, setCustomText] = useState<string[]>(
    request.questions.map(() => ""),
  );

  const toggle = (qi: number, label: string, multiple?: boolean) => {
    setSelected((prev) => {
      const next = prev.map((a) => [...a]);
      if (multiple) {
        const i = next[qi].indexOf(label);
        if (i >= 0) next[qi].splice(i, 1);
        else next[qi].push(label);
      } else {
        next[qi] = [label];
      }
      return next;
    });
  };

  const submit = () => {
    const answers = request.questions.map((q, qi) => {
      const a = [...selected[qi]];
      const ct = customText[qi].trim();
      if (q.custom && ct) a.push(ct);
      return a;
    });
    onReply(answers);
  };

  const canSubmit = request.questions.every(
    (_, i) => selected[i].length > 0 || customText[i].trim(),
  );

  return (
    <div className="question-prompt">
      <div className="question-prompt-label">
        <HelpCircle size={16} /> The assistant is asking
      </div>
      {request.questions.map((q, qi) => (
        <div key={qi} className="question-block">
          {q.header && <div className="question-header">{q.header}</div>}
          <div className="question-text">{q.question}</div>
          <div className="question-options">
            {q.options.map((opt) => (
              <button
                key={opt.label}
                className={`question-option ${
                  selected[qi].includes(opt.label) ? "selected" : ""
                }`}
                onClick={() => toggle(qi, opt.label, q.multiple)}
                title={opt.description}
              >
                {opt.label}
              </button>
            ))}
          </div>
          {q.custom && (
            <input
              className="question-custom"
              placeholder="Or type your own answer…"
              value={customText[qi]}
              onChange={(e) =>
                setCustomText((prev) => {
                  const n = [...prev];
                  n[qi] = e.target.value;
                  return n;
                })
              }
            />
          )}
        </div>
      ))}
      <div className="question-actions">
        <button className="btn-reject" onClick={onReject}>
          Skip
        </button>
        <button className="btn-answer" onClick={submit} disabled={!canSubmit}>
          <Send size={16} /> Answer
        </button>
      </div>
    </div>
  );
}
