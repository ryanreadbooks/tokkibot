package dataurl

import (
	"encoding/base64"
	"strings"

	"github.com/gabriel-vasile/mimetype"
)

func Base64Encode(data []byte) string {
	mimeType := mimetype.Detect(data)
	// data:[<media type>][;base64],<data>
	encodedData := base64.StdEncoding.EncodeToString(data)
	var builder strings.Builder
	builder.Grow(len(encodedData) + len(mimeType.String()) + 10)
	builder.WriteString("data:")
	builder.WriteString(mimeType.String())
	builder.WriteString(";base64,")
	builder.WriteString(encodedData)
	return builder.String()
}
