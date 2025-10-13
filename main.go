package main

import (
	"context"
	"flag"
	"log"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type EchoArgs struct {
	Message string `json:"message"`
}

func Echo(ctx context.Context, req *mcp.CallToolRequest, args *EchoArgs) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "Echo: " + args.Message},
		},
	}, nil, nil
}

func main() {
	// Parse command line flags
	authzServerURL := flag.String("authz-server-url", "http://localhost/realms/demo", "Authorization Server URL")
	jwksURL := flag.String("jwks-url", "http://localhost/realms/demo/protocol/openid-connect/certs", "JWKS URL")
	resourceURL := flag.String("resource-url", "http://localhost:8000", "Resource URL for this server")
	flag.Parse()

	// Initialize OAuth config
	oauthConfig := &OAuthConfig{
		AuthzServerURL: *authzServerURL,
		JwksURL:        *jwksURL,
		ResourceURL:    *resourceURL,
	}

	if err := oauthConfig.InitJWKS(); err != nil {
		log.Fatalf("Failed to initialize JWKS: %v", err)
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "simple-mcp-server",
		Version: "1.0.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "echo",
		Description: "Echoes back the input message",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{
					"type":        "string",
					"description": "The message to echo back",
				},
			},
			"required": []string{"message"},
		},
	}, Echo)

	// MCP handler
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, nil)

	// Setup routing
	mux := http.NewServeMux()

	// OAuth 2.1 metadata endpoint (no authorization required)
	mux.HandleFunc("/.well-known/oauth-protected-resource", oauthConfig.HandleProtectedResourceMetadata)

	// MCP endpoint (OAuth authorization required, with logging)
	mux.Handle("/", LoggingMiddleware(oauthConfig.OAuthMiddleware(mcpHandler)))

	log.Println("Starting MCP server on :8000")
	log.Printf("Authorization Server URL: %s", *authzServerURL)
	log.Printf("JWKS URL: %s", *jwksURL)
	log.Printf("Resource URL: %s", *resourceURL)
	log.Println("Tool available: echo")
	log.Println("OAuth2.1 endpoint:")
	log.Println("  - /.well-known/oauth-protected-resource")

	if err := http.ListenAndServe(":8000", mux); err != nil {
		log.Printf("Server failed: %v", err)
	}
}
