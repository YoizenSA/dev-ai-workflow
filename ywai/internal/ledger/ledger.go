package ledger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	dirName  = ".ywai"
	fileName = "ledger.json"
	maxCore  = 2
)

// LeakMarkers must never appear in a file passed to ship. They are the inner
// register the skill uses while thinking; outer text stays clean.
var LeakMarkers = []string{"// ledger:"}

type Ledger struct {
	Goal      string     `json:"goal"`
	Core      []string   `json:"core"`
	Next      string     `json:"next"`
	Open      []Question `json:"open"`
	Verified  []Check    `json:"verified"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

type Question struct {
	ID        int    `json:"id"`
	Text      string `json:"text"`
	SettledBy string `json:"settledBy,omitempty"`
	Closed    bool   `json:"closed,omitempty"`
}

type Check struct {
	Text string    `json:"text"`
	By   string    `json:"by"`
	At   time.Time `json:"at"`
}

type Note struct {
	Goal      string
	Next      string
	Core      string
	CoreSlot  int // 1-based; 0 appends
	Check     string
	By        string
	Open      string
	SettledBy string
	Close     int // 0 means none
}

func Path(cwd string) string {
	return filepath.Join(cwd, dirName, fileName)
}

func Load(cwd string) (*Ledger, error) {
	data, err := os.ReadFile(Path(cwd))
	if err != nil {
		if os.IsNotExist(err) {
			return &Ledger{}, nil
		}
		return nil, fmt.Errorf("read ledger: %w", err)
	}
	var l Ledger
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, fmt.Errorf("parse ledger: %w", err)
	}
	return &l, nil
}

func Save(cwd string, l *Ledger) error {
	if l == nil {
		return fmt.Errorf("nil ledger")
	}
	l.UpdatedAt = time.Now().UTC()
	dir := filepath.Join(cwd, dirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create ledger dir: %w", err)
	}
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal ledger: %w", err)
	}
	return os.WriteFile(Path(cwd), append(data, '\n'), 0o644)
}

func ApplyNote(l *Ledger, n Note) error {
	if l == nil {
		return fmt.Errorf("nil ledger")
	}
	if strings.TrimSpace(n.Goal) != "" {
		l.Goal = strings.TrimSpace(n.Goal)
	}
	if strings.TrimSpace(n.Next) != "" {
		l.Next = strings.TrimSpace(n.Next)
	}
	if strings.TrimSpace(n.Core) != "" {
		if err := setCore(l, strings.TrimSpace(n.Core), n.CoreSlot); err != nil {
			return err
		}
	}
	if strings.TrimSpace(n.Open) != "" {
		nextID := 1
		for _, q := range l.Open {
			if q.ID >= nextID {
				nextID = q.ID + 1
			}
		}
		l.Open = append(l.Open, Question{
			ID:        nextID,
			Text:      strings.TrimSpace(n.Open),
			SettledBy: strings.TrimSpace(n.SettledBy),
		})
	}
	if n.Close != 0 {
		if err := closeQuestion(l, n.Close, n.Check, n.By); err != nil {
			return err
		}
		return nil
	}
	if strings.TrimSpace(n.Check) != "" {
		return appendCheck(l, n.Check, n.By)
	}
	return nil
}

func setCore(l *Ledger, item string, slot int) error {
	if slot < 0 || slot > maxCore {
		return fmt.Errorf("--core-slot must be 1 or %d", maxCore)
	}
	if slot > 0 {
		for len(l.Core) < slot {
			l.Core = append(l.Core, "")
		}
		l.Core[slot-1] = item
		return nil
	}
	if len(l.Core) >= maxCore {
		return fmt.Errorf("core is full (%d items); pass --core-slot 1 or %d to swap", maxCore, maxCore)
	}
	l.Core = append(l.Core, item)
	return nil
}

func appendCheck(l *Ledger, text, by string) error {
	if strings.TrimSpace(by) == "" {
		return fmt.Errorf("checkpoint requires --by (verifier and coverage)")
	}
	l.Verified = append(l.Verified, Check{
		Text: strings.TrimSpace(text),
		By:   strings.TrimSpace(by),
		At:   time.Now().UTC(),
	})
	return nil
}

func closeQuestion(l *Ledger, id int, check, by string) error {
	if strings.TrimSpace(check) == "" || strings.TrimSpace(by) == "" {
		return fmt.Errorf("close requires --check and --by")
	}
	found := false
	for i := range l.Open {
		if l.Open[i].ID == id {
			l.Open[i].Closed = true
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("no open question %d", id)
	}
	return appendCheck(l, check, by)
}

func isEmpty(l *Ledger) bool {
	if l == nil {
		return true
	}
	return l.Goal == "" && l.Next == "" && len(l.Core) == 0 && len(l.Open) == 0 && len(l.Verified) == 0
}

func RenderSeam(l *Ledger) string {
	if isEmpty(l) {
		return "no ledger\n"
	}
	return render(l)
}

func RenderResume(l *Ledger) string {
	var b strings.Builder
	b.WriteString("Resume from the ledger. Restate the pass (solo|thin|full) and make Next the first action back.\n\n")
	if isEmpty(l) {
		b.WriteString("no ledger\n")
		return b.String()
	}
	b.WriteString(render(l))
	return b.String()
}

func render(l *Ledger) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Goal: %s\n", l.Goal)
	b.WriteString("Core:\n")
	if len(l.Core) == 0 {
		b.WriteString("  (none)\n")
	}
	for i, c := range l.Core {
		fmt.Fprintf(&b, "  %d. %s\n", i+1, c)
	}
	fmt.Fprintf(&b, "Next: %s\n", l.Next)
	b.WriteString("Open:\n")
	openCount := 0
	for _, q := range l.Open {
		if q.Closed {
			continue
		}
		openCount++
		fmt.Fprintf(&b, "  #%d %s", q.ID, q.Text)
		if q.SettledBy != "" {
			fmt.Fprintf(&b, " (settled by %s)", q.SettledBy)
		}
		b.WriteByte('\n')
	}
	if openCount == 0 {
		b.WriteString("  (none)\n")
	}
	b.WriteString("Verified:\n")
	if len(l.Verified) == 0 {
		b.WriteString("  (none)\n")
	}
	for _, c := range l.Verified {
		fmt.Fprintf(&b, "  - %s (by %s)\n", c.Text, c.By)
	}
	return b.String()
}

func ShipFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read ship file: %w", err)
	}
	content := string(data)
	for _, marker := range LeakMarkers {
		if strings.Contains(content, marker) {
			return fmt.Errorf("ship refused: inner register leaked %q in %s", marker, path)
		}
	}
	return nil
}
