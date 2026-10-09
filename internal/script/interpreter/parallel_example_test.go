package interpreter

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"sync"
	"testing"

	"github.com/kunchenguid/gsh/internal/script/lexer"
	"github.com/kunchenguid/gsh/internal/script/parser"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// Run the shipped script through the real MCP loader and tool dispatch, without
// making CI depend on an external service. The transport observes the original
// URL and headers before routing requests to a local Streamable HTTP server.
func TestParallelSearchExample(t *testing.T) {
	script, err := os.ReadFile("../../../docs/script/examples/parallel-search.gsh")
	require.NoError(t, err)

	server := mcp.NewServer(&mcp.Implementation{Name: "parallel-fixture", Version: "1.0.0"}, nil)
	type arguments struct {
		Objective     string   `json:"objective"`
		SearchQueries []string `json:"search_queries,omitempty"`
		URLs          []string `json:"urls,omitempty"`
		SessionID     string   `json:"session_id"`
	}
	var calls []arguments
	for _, name := range []string{"web_search", "web_fetch"} {
		mcp.AddTool(server, &mcp.Tool{Name: name}, func(_ context.Context, req *mcp.CallToolRequest, input arguments) (*mcp.CallToolResult, any, error) {
			calls = append(calls, input)
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: req.Params.Name + " excerpt: https://go.dev/doc/"}}}, nil, nil
		})
	}
	local := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server { return server }, nil))
	defer local.Close()
	localURL, err := url.Parse(local.URL)
	require.NoError(t, err)
	transport := &parallelExampleTransport{base: http.DefaultTransport, localURL: localURL}
	http.DefaultTransport = transport
	defer func() { http.DefaultTransport = transport.base }()

	p := parser.New(lexer.New(string(script)))
	program := p.ParseProgram()
	require.Empty(t, p.Errors())
	interp := New(nil)
	defer interp.Close()
	result, err := interp.Eval(program)
	require.NoError(t, err)
	require.Contains(t, result.Variables()["searchResult"].String(), "web_search excerpt")
	require.Contains(t, result.Variables()["fetchResult"].String(), "web_fetch excerpt")
	require.Len(t, calls, 2)
	require.NotEmpty(t, calls[0].Objective)
	require.NotEmpty(t, calls[0].SearchQueries)
	require.Equal(t, []string{"https://go.dev/doc/"}, calls[1].URLs)
	require.Regexp(t, regexp.MustCompile(`^[0-9a-f]{32}$`), calls[0].SessionID)
	require.Equal(t, calls[0].SessionID, calls[1].SessionID)

	transport.mu.Lock()
	defer transport.mu.Unlock()
	for _, method := range []string{"initialize", "tools/list", "tools/call:web_search", "tools/call:web_fetch"} {
		require.Contains(t, transport.methods, method)
	}
}

type parallelExampleTransport struct {
	base     http.RoundTripper
	localURL *url.URL
	mu       sync.Mutex
	methods  []string
}

func (t *parallelExampleTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.String() != "https://search.parallel.ai/mcp" {
		return nil, &url.Error{Op: "request", URL: req.URL.String(), Err: os.ErrInvalid}
	}
	if req.Header.Get("User-Agent") != "gsh (https://github.com/kunchenguid/gsh)" || req.Header.Get("Authorization") != "" {
		return nil, os.ErrPermission
	}
	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(body))
		var message struct {
			Method string `json:"method"`
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		if err := json.Unmarshal(body, &message); err != nil {
			return nil, err
		}
		method := message.Method
		if method == "tools/call" {
			method += ":" + message.Params.Name
		}
		t.mu.Lock()
		t.methods = append(t.methods, method)
		t.mu.Unlock()
	}
	copy := req.Clone(req.Context())
	copy.URL.Scheme = t.localURL.Scheme
	copy.URL.Host = t.localURL.Host
	return t.base.RoundTrip(copy)
}
