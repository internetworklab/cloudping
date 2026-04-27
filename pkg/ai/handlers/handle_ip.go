package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"slices"

	pkgipinfo "github.com/internetworklab/cloudping/pkg/ipinfo"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type IPQueryHandler struct {
	ToolName               string
	ToolDescription        string
	IPInfoProviderRegistry *pkgipinfo.IPInfoProviderRegistry
}

// Using the generic AddTool automatically populates the the input and output
// schema of the tool.
type IPQueryArgs struct {
	IP              string   `json:"ip" jsonschema:"The ip or ipv6 address to query."`
	IPInfoProviders []string `json:"ipinfo_providers,omitempty" jsonschema:"The list of ipinfo providers to use, if it's nil or empty, all available providers will use."`
}

func (handler *IPQueryHandler) GetName() string {
	if name := handler.ToolName; name != "" {
		return name
	}
	return "ip-query"
}

func (handler *IPQueryHandler) GetDescription() string {
	if description := handler.ToolDescription; description != "" {
		return description
	}
	return "Query informations about the given IP, for example: ASN and geographic location."
}

func (handler *IPQueryHandler) GetTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        handler.GetName(),
		Description: handler.GetDescription(),
	}
}

type IPQueryResultEntry struct {
	FromProvider string `json:"from_provider"`
	QueryIP      string `json:"query_ip"`
	Result       any    `json:"query_result"`
	Error        string `json:"err,omitempty"`
}

type IPQueryResult struct {
	Error string `json:"err,omitempty"`
	Data  []IPQueryResultEntry
}

func (handler *IPQueryHandler) HandleToolRequest(ctx context.Context, req *mcp.CallToolRequest, args IPQueryArgs) (*mcp.CallToolResult, IPQueryResult, error) {
	logRequest(ctx, args, handler.GetName())

	results := make([]IPQueryResultEntry, 0)

	for _, name := range handler.IPInfoProviderRegistry.GetRegisteredAdapterNames() {
		if len(args.IPInfoProviders) > 0 && slices.Index(args.IPInfoProviders, name) == -1 {
			continue
		}

		provider, err := handler.IPInfoProviderRegistry.GetAdapter(name)
		if err != nil {
			log.Printf("[err] failed to get adapter: %v", err)
			continue
		}

		ans, err := provider.GetIPInfo(ctx, args.IP)
		if err != nil {
			result := IPQueryResultEntry{
				FromProvider: name,
				QueryIP:      args.IP,
				Error:        err.Error(),
			}
			results = append(results, result)
			continue
		}

		result := IPQueryResultEntry{
			FromProvider: name,
			QueryIP:      args.IP,
			Result:       ans,
		}
		results = append(results, result)
	}

	j, err := json.Marshal(results)
	if err != nil {
		log.Printf("[err] failed to marshal results: %v", err)
		return nil, IPQueryResult{}, err
	}

	mcpToolCallResult := &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(j)},
		},
	}

	return mcpToolCallResult, IPQueryResult{Data: results}, nil
}

func (handler *IPQueryHandler) RegisterTool(mcpsrv *mcp.Server) {
	mcp.AddTool(mcpsrv, handler.GetTool(), handler.HandleToolRequest)
}

func (handler *IPQueryHandler) getResourceURI() string {
	return "config://ipinfo-providers"
}

func (handler *IPQueryHandler) getResourceMIMEType() string {
	return "application/json"
}

func (handler *IPQueryHandler) getResourceName() string {
	return "IPInfoProviderList"
}

func (handler *IPQueryHandler) getResourceDescription() string {
	return `The list of currently supporting ipinfo providers.
When querying ip informations, the user might specify what ipinfo provider to use, or leave it to empty to use all available providers, usually it won't be too much.
returns a list of string.`
}

func (handler *IPQueryHandler) HandleResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	providers := handler.IPInfoProviderRegistry.GetRegisteredAdapterNames()
	data, err := json.Marshal(providers)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize ipinfo provider list: %w", err)
	}

	contents := make([]*mcp.ResourceContents, 0)
	content := &mcp.ResourceContents{
		URI:      handler.getResourceURI(),
		MIMEType: handler.getResourceMIMEType(),
		Text:     string(data),
	}
	contents = append(contents, content)

	return &mcp.ReadResourceResult{Contents: contents}, nil
}

func (handler *IPQueryHandler) RegisterResource(mcpsrv *mcp.Server) {
	resourceDecl := &mcp.Resource{
		URI:         handler.getResourceURI(),
		Name:        handler.getResourceName(),
		MIMEType:    handler.getResourceMIMEType(),
		Description: handler.getResourceDescription(),
	}

	mcpsrv.AddResource(resourceDecl, handler.HandleResource)
}
