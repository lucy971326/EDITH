package volume

import (
	"crypto/sha256"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

// EDITH_E2B_DEBUG_HTTP enables short-lived diagnostics for the E2B Volume
// boundary. Never enable it as a substitute for normal application logging.
const traceEnv = "EDITH_E2B_DEBUG_HTTP"

func traceEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(traceEnv)), "true")
}

type volumeTraceTransport struct {
	next http.RoundTripper
}

func (t volumeTraceTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	next := t.next
	if next == nil {
		next = http.DefaultTransport
	}

	log.Printf("e2b volume request method=%s host=%s path=%s authorization=%s api_key=%t",
		request.Method,
		request.URL.Host,
		request.URL.EscapedPath(),
		authorizationSummary(request.Header.Get("Authorization")),
		request.Header.Get("X-API-Key") != "",
	)
	response, err := next.RoundTrip(request)
	if err != nil {
		log.Printf("e2b volume response host=%s path=%s transport_error=%q", request.URL.Host, request.URL.EscapedPath(), err)
		return nil, err
	}
	log.Printf("e2b volume response host=%s path=%s status=%d request_id=%q trace_id=%q",
		request.URL.Host,
		request.URL.EscapedPath(),
		response.StatusCode,
		requestID(response.Header),
		response.Header.Get("Traceparent"),
	)
	return response, nil
}

func authorizationSummary(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "missing"
	}
	parts := strings.Fields(value)
	if len(parts) != 2 {
		return "malformed"
	}
	return fmt.Sprintf("%s token_sha256=%x token_len=%d", parts[0], sha256.Sum256([]byte(parts[1])), len(parts[1]))
}

func requestID(headers http.Header) string {
	for _, key := range []string{"X-Request-ID", "X-Request-Id", "Request-ID", "Request-Id"} {
		if value := headers.Get(key); value != "" {
			return value
		}
	}
	return ""
}
