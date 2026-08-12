package api

import (
	"bytes"
	"testing"

	appjira "kode-stream/internal/jira"
)

func TestCopyBoundedAttachmentNeverWritesPastLimit(t *testing.T) {
	var target bytes.Buffer
	input := bytes.Repeat([]byte("x"), int(appjira.MaxAttachmentBytes)+1)
	err := copyBoundedAttachment(&target, bytes.NewReader(input))
	if err == nil || int64(target.Len()) > appjira.MaxAttachmentBytes {
		t.Fatalf("err=%v bytes=%d", err, target.Len())
	}
}
