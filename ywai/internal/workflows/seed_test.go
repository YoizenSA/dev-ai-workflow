package workflows

import "testing"

func TestDiffSeed(t *testing.T) {
	const local = `{
		"id": "ship", "name": "ship", "version": "1.0.0",
		"updatedAt": "2026-07-25T00:00:00Z",
		"nodes": [
			{"id":"start","type":"start","name":"start","position":{"x":0,"y":0},"data":{"label":"Start"}},
			{"id":"build","type":"subAgent","name":"builder","position":{"x":1,"y":0},"data":{"prompt":"old prompt"}},
			{"id":"legacy","type":"prompt","name":"legacy-step","position":{"x":2,"y":0},"data":{"prompt":"gone in seed"}}
		]
	}`
	const seed = `{
		"id": "ship", "name": "ship", "version": "1.1.0",
		"updatedAt": "2026-09-09T00:00:00Z",
		"nodes": [
			{"id":"start","type":"start","name":"start","position":{"x":0,"y":0},"data":{"label":"Start"}},
			{"id":"build","type":"subAgent","name":"builder","position":{"x":1,"y":0},"data":{"prompt":"new prompt"}},
			{"id":"verify","type":"subAgentFlow","name":"scenario-verification","position":{"x":3,"y":0},"data":{"flowId":"scenario-verification"}}
		]
	}`

	t.Run("drift: added, removed and changed nodes", func(t *testing.T) {
		d := DiffSeed([]byte(local), []byte(seed))
		if d.InSync {
			t.Fatal("expected drift, got in-sync")
		}
		if d.Version != "1.1.0" || d.UpdatedAt != "2026-09-09T00:00:00Z" {
			t.Fatalf("seed meta not carried: %+v", d)
		}
		if len(d.AddedNodes) != 1 || d.AddedNodes[0] != "scenario-verification" {
			t.Fatalf("added: %v", d.AddedNodes)
		}
		if len(d.RemovedNodes) != 1 || d.RemovedNodes[0] != "legacy-step" {
			t.Fatalf("removed: %v", d.RemovedNodes)
		}
		if len(d.ChangedNodes) != 1 || d.ChangedNodes[0] != "builder" {
			t.Fatalf("changed: %v", d.ChangedNodes)
		}
	})

	t.Run("identical designs are in sync (layout ignored)", func(t *testing.T) {
		moved := `{
			"id": "ship", "name": "ship", "version": "1.1.0",
			"updatedAt": "2026-09-09T00:00:00Z",
			"nodes": [
				{"id":"start","type":"start","name":"start","position":{"x":99,"y":99},"data":{"label":"Start"}},
				{"id":"build","type":"subAgent","name":"builder","position":{"x":5,"y":5},"data":{"prompt":"new prompt"}},
				{"id":"verify","type":"subAgentFlow","name":"scenario-verification","position":{"x":7,"y":7},"data":{"flowId":"scenario-verification"}}
			]
		}`
		d := DiffSeed([]byte(moved), []byte(seed))
		if !d.InSync {
			t.Fatalf("moved nodes only should be in-sync: %+v", d)
		}
	})

	t.Run("unparseable input claims nothing", func(t *testing.T) {
		d := DiffSeed([]byte("{bogus"), []byte(seed))
		if !d.InSync || d.Version != "" {
			t.Fatalf("expected empty in-sync diff: %+v", d)
		}
	})
}
