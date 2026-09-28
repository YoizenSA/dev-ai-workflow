package control

import (
	"bytes"
	"testing"
)

func TestWebPushRecordIsReadableByTheBrowser(t *testing.T) {
	payload := []byte(`{"title":"ywai Test","body":"Push notifications are working!"}`)
	record := padWebPushRecord(payload)
	if len(record) == 0 || record[len(record)-1] != 0x02 {
		t.Fatalf("record must end with the aes128gcm delimiter, got %x", record)
	}
	if !bytes.Equal(record[:len(record)-1], payload) {
		t.Fatalf("browser would not see the notification JSON, got %q", record)
	}
}
