# OAuth-Protected MCP Server Example (Go)

Sample implementation of a Model Context Protocol (MCP) server protected by OAuth 2.1, written in Go.

This project demonstrates how to implement an MCP server with OAuth protection using the [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk) and [Keycloak](https://www.keycloak.org/) as the authorization server.

## Features

- **OAuth 2.0 Protected Resource**: Implements [RFC 9728 (OAuth 2.0 Protected Resource Metadata)](https://datatracker.ietf.org/doc/html/rfc9728)
- **JWT Access Token Validation**: Local validation using JWKS
- **Streamable HTTP Transport**: Remote-accessible MCP server
- **Simple Echo Tool**: Basic MCP tool for demonstration
- **Keycloak Integration**: Uses Keycloak 26.4 as authorization server with Dynamic Client Registration (DCR)

## Architecture

```
                    OAuth 2.1 Flow (DCR, Authorization Code)
    ┌───────────────────────────────────────────────────────────┐
    │                                                           │
    │                                                           ▼
┌───┴─────────┐   HTTP + Bearer Token   ┌─────────────┐   ┌───────────────┐
│ MCP Client  │────────────────────────►│ MCP Server  │   │  Keycloak     │
│ (Inspector) │                         │ (This repo) │   │ (AuthZ Server)│
└─────────────┘                         └─────────────┘   └───────────────┘
                                              │                   │
                                              │◄──────────────────┘
                                              │   JWKS (RS256 Public Key)
                                              │
                                              ▼
                                    JWT Access Token Validation:
                                    • Signature verification
                                    • iss, exp, aud claims
                                    • scope claim
```

## Prerequisites

- Go 1.25 or later
- Docker & Docker Compose (for running Keycloak)

## Quick Start

### 1. Start Keycloak

```bash
cd authz-server
docker-compose up -d
```

Keycloak will be available at `http://localhost` (admin/admin).

### 2. Configure Keycloak

1. Create a new realm named `demo`
2. Create a client scope named `mcp:tools`:
   - Include in token scope: `On`
   - Add Audience mapper:
     - Name: `audience-config`
     - Included Custom Audience: `http://localhost:8000`
3. Configure Client Policies:
   - Delete the default "Trusted Hosts" policy
   - Update "Allowed Client Scopes" policy to include `mcp:tools`
4. Create a test user


### 3. Run MCP Server

```bash
go run . \
  -authz-server-url="http://localhost/realms/demo" \
  -jwks-url="http://localhost/realms/demo/protocol/openid-connect/certs" \
  -resource-url="http://localhost:8000"
```

### 4. Test with MCP Inspector

Run [MCP Inspector](https://github.com/modelcontextprotocol/inspector) and connect to `http://localhost:8000`.

The MCP Inspector will:
1. Fetch Protected Resource Metadata from `http://localhost:8000/.well-known/oauth-protected-resource`
2. Discover authorization server metadata
3. Register as a client using Dynamic Client Registration (DCR)
4. Initiate OAuth authorization code flow
5. Redirect you to Keycloak login page
6. Obtain access token and connect to MCP server

## Project Structure

```
.
├── authz-server/              # Keycloak setup
│   ├── docker-compose.yml
│   └── nginx.conf
├── main.go                    # MCP server implementation
├── oauth_middleware.go        # OAuth middleware & JWT Access Token validation
└── README.md
```

## Implementation Highlights

### OAuth 2.0 Protected Resource Metadata

The server exposes metadata at `/.well-known/oauth-protected-resource`:

```json
{
  "resource": "http://localhost:8000",
  "authorization_servers": ["http://localhost/realms/demo"],
  "scopes_supported": ["mcp:tools"]
}
```

### JWT Access Token Validation

The middleware validates:

1. **Signature**: Using JWKS from authorization server (RS256)
2. **Standard Claims**:
   - `iss` (issuer): Must match authorization server URL
   - `exp` (expiration): Token must not be expired
   - `aud` (audience): Must include this server's URL
3. **Custom Claims**:
   - `scope`: Must include `mcp:tools`

### MCP Tool

Provides a simple `echo` tool that returns the input message.

## Configuration Options

| Flag | Description | Default |
|------|-------------|---------|
| `-authz-server-url` | Authorization server URL | `http://localhost/realms/demo` |
| `-jwks-url` | JWKS endpoint URL | `http://localhost/realms/demo/protocol/openid-connect/certs` |
| `-resource-url` | This server's URL | `http://localhost:8000` |

## Limitations & Notes

### RFC 8707 Support

Keycloak 26.4 does not yet support [RFC 8707 (Resource Indicators for OAuth 2.0)](https://datatracker.ietf.org/doc/html/rfc8707). As a workaround, this implementation uses an Audience Mapper in the `mcp:tools` scope to set the `aud` claim.

### CORS Workaround

Due to a [known issue in Keycloak 26.4](https://github.com/keycloak/keycloak/issues/39629), nginx is used as a reverse proxy to add CORS headers for the DCR endpoint.

## Related Specifications

- [MCP Specification 2025-06-18 - Authorization](https://modelcontextprotocol.io/specification/2025-06-18/basic/authorization)
- [RFC 9728: OAuth 2.0 Protected Resource Metadata](https://datatracker.ietf.org/doc/html/rfc9728)
- [RFC 9068: OAuth 2.0 Access Token in JWT Format](https://datatracker.ietf.org/doc/html/rfc9068)
- [RFC 8707: Resource Indicators for OAuth 2.0](https://datatracker.ietf.org/doc/html/rfc8707)

## Resources

- [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)
- [MCP Inspector](https://github.com/modelcontextprotocol/inspector)
- [Keycloak](https://www.keycloak.org/)

## License

MIT
