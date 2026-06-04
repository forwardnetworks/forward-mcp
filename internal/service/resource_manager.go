package service

import (
	"github.com/forward-mcp/internal/logger"
	mcp "github.com/metoro-io/mcp-golang"
)

// ResourceManager registers MCP contextual resources for the Forward Networks service.
type ResourceManager struct {
	service *ForwardMCPService
	logger  *logger.Logger
}

// NewResourceManager creates a ResourceManager backed by the given service.
func NewResourceManager(s *ForwardMCPService, log *logger.Logger) *ResourceManager {
	return &ResourceManager{service: s, logger: log}
}

// RegisterAllResources registers all contextual resources with the MCP server.
func (r *ResourceManager) RegisterAllResources(server *mcp.Server) error {
	return nil
}
