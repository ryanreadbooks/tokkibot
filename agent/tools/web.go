package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/ryanreadbooks/tokkibot/component/tool"
	"github.com/ryanreadbooks/tokkibot/pkg/xstring"

	"github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
)

const (
	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36"

	maxWebFetchOutputChars = 50000
	maxContentLength       = 10 * 1024 * 1024 // 10MB
)

var (
	httpClientOnce = sync.Once{}
	httpClient     *http.Client

	stripJsScriptRegexp = regexp.MustCompile(`<script[\s\S]*?</script>`)
	stripStyleRegexp    = regexp.MustCompile(`<style[\s\S]*?</style>`)
	stripNewlinesRegexp = regexp.MustCompile(`\n{3,}`)
)

func getHttpClient() *http.Client {
	httpClientOnce.Do(func() {
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return errors.New("stopped after 5 redirects")
				}
				return nil
			},
			Transport: &http.Transport{
				IdleConnTimeout:       5 * time.Minute,
				ResponseHeaderTimeout: 10 * time.Second,
			},
		}
	})
	return httpClient
}

type WebFetchInput struct {
	URL string `json:"url" jsonschema:"description=The URL to fetch"`
}

type WebFetchOutput struct {
	Content     string `json:"content"`
	Truncated   bool   `json:"truncated"`
	IsBinary    bool   `json:"is_binary"`
	ContentType string `json:"content_type"`
	StatusCode  int    `json:"status_code"`
}

func newWebFetchTextOutput(content string, contentType string, statusCode int) *WebFetchOutput {
	if l := utf8.RuneCountInString(content); l > maxWebFetchOutputChars {
		return &WebFetchOutput{
			Content:     xstring.Truncate(content, maxWebFetchOutputChars) + "...",
			Truncated:   true,
			IsBinary:    false,
			ContentType: contentType,
			StatusCode:  statusCode,
		}
	}

	return &WebFetchOutput{
		Content:     content,
		Truncated:   false,
		IsBinary:    false,
		ContentType: contentType,
		StatusCode:  statusCode,
	}
}

func newWebFetchBinaryOutput(data []byte, contentType string, statusCode int) *WebFetchOutput {
	truncated := len(data) > maxWebFetchOutputChars
	if truncated {
		data = data[:maxWebFetchOutputChars]
	}

	return &WebFetchOutput{
		Content:     base64.URLEncoding.EncodeToString(data),
		Truncated:   truncated,
		IsBinary:    true,
		ContentType: contentType,
		StatusCode:  statusCode,
	}
}

func WebFetch() tool.Invoker {
	return tool.NewInvoker(tool.Info{
		Name:        ToolNameWebFetch,
		Description: "Fetch URL and extract HTML content to markdown or text",
	}, func(ctx context.Context, meta tool.InvokeMeta, input *WebFetchInput) (*WebFetchOutput, error) {
		// Validate URL scheme
		if !strings.HasPrefix(input.URL, "http://") && !strings.HasPrefix(input.URL, "https://") {
			return nil, errors.New("invalid URL scheme, only http:// and https:// are supported")
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, input.URL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", userAgent)

		domain := req.URL.Hostname()

		resp, err := getHttpClient().Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		// Check HTTP status code
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
		}

		// Check content length to avoid memory issues
		if resp.ContentLength > maxContentLength {
			return nil, fmt.Errorf("content too large: %d bytes (max %d bytes)", resp.ContentLength, maxContentLength)
		}

		// Read response body with size limit
		limitedReader := io.LimitReader(resp.Body, maxContentLength)
		body, err := io.ReadAll(limitedReader)
		if err != nil {
			return nil, err
		}

		contentType := resp.Header.Get("Content-Type")
		if contentType == "" {
			contentType = http.DetectContentType(body)
		}

		// Handle HTML content
		if strings.Contains(contentType, "text/html") {
			markdown, err := convertHTMLToMarkdown(ctx, body, domain)
			if err != nil {
				return nil, err
			}
			return newWebFetchTextOutput(markdown, contentType, resp.StatusCode), nil
		}

		// Handle text content types
		if isTextContent(contentType) {
			return newWebFetchTextOutput(string(body), contentType, resp.StatusCode), nil
		}

		// Handle binary content
		return newWebFetchBinaryOutput(body, contentType, resp.StatusCode), nil
	})
}

// isTextContent checks if the content type is text-based
func isTextContent(contentType string) bool {
	textTypes := []string{
		"text/plain",
		"text/xml",
		"text/css",
		"text/javascript",
		"application/json",
		"application/xml",
		"application/javascript",
		"application/yaml",
		"application/x-yaml",
	}

	for _, t := range textTypes {
		if strings.Contains(contentType, t) {
			return true
		}
	}
	return false
}

// convertHTMLToMarkdown converts HTML to markdown and cleans up the content
func convertHTMLToMarkdown(ctx context.Context, body []byte, domain string) (string, error) {
	// Strip js script tags and style tags with regex
	body = stripJsScriptRegexp.ReplaceAll(body, []byte(""))
	body = stripStyleRegexp.ReplaceAll(body, []byte(""))

	// Convert html to markdown
	markdown, err := htmltomarkdown.ConvertReader(bytes.NewReader(body),
		converter.WithContext(ctx),
		converter.WithDomain(domain),
	)
	if err != nil {
		return "", err
	}

	// Clean up excessive newlines
	result := string(markdown)
	result = stripNewlinesRegexp.ReplaceAllString(result, "\n\n")

	return strings.TrimSpace(result), nil
}
