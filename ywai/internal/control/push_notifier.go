package control

import (
	"fmt"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/missions"
)

// newFeaturePushSink returns a mission event sink that sends a push
// notification when a feature reaches a terminal state. It replaces the
// completion hook that used to hang off the kanban projector before it was removed.
func newFeaturePushSink(send func(title, body string) error) func(evtType string, payload interface{}) {
	return func(evtType string, payload interface{}) {
		if evtType != "feature_status_changed" || send == nil {
			return
		}
		payloadMap, ok := payload.(map[string]interface{})
		if !ok {
			return
		}

		// FeatureStatus is a named string type, so a direct .(string)
		// assertion fails on the dynamic type. Normalize instead.
		var status string
		switch v := payloadMap["status"].(type) {
		case missions.FeatureStatus:
			status = string(v)
		case string:
			status = v
		default:
			status = fmt.Sprintf("%v", payloadMap["status"])
		}

		switch missions.FeatureStatus(status) {
		case missions.FeatureCompleted:
			_ = send("ywai: Task Complete", "A task finished successfully")
		case missions.FeatureFailed:
			_ = send("ywai: Task Failed", "A task encountered an error")
		}
	}
}
