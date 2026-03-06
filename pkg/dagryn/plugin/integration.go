package plugin

import (
	"context"
	"sync"
)

// IntegrationPlugin represents a registered integration plugin.
type IntegrationPlugin struct {
	Name     string
	Manifest *Manifest
	Inputs   map[string]string
}

// IntegrationRegistry manages integration plugins and dispatches lifecycle hooks.
type IntegrationRegistry struct {
	plugins  []IntegrationPlugin
	executor *HookExecutor
	mu       sync.RWMutex
}

// NewIntegrationRegistry creates a new IntegrationRegistry.
func NewIntegrationRegistry(executor *HookExecutor) *IntegrationRegistry {
	return &IntegrationRegistry{
		plugins:  make([]IntegrationPlugin, 0),
		executor: executor,
	}
}

// Register adds an integration plugin to the registry.
func (r *IntegrationRegistry) Register(p IntegrationPlugin) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plugins = append(r.plugins, p)
}

// DispatchHook runs the named hook across all registered integration plugins.
// Failures are logged but non-fatal.
func (r *IntegrationRegistry) DispatchHook(ctx context.Context, hookName string, hctx *HookContext) {
	r.mu.RLock()
	plugins := make([]IntegrationPlugin, len(r.plugins))
	copy(plugins, r.plugins)
	r.mu.RUnlock()

	for _, p := range plugins {
		hook, ok := p.Manifest.Hooks[hookName]
		if !ok {
			continue
		}
		if err := r.executor.RunHook(ctx, p.Name, hookName, hook, p.Inputs, hctx); err != nil {
			r.executor.logger.Warn("integration hook failed (non-fatal): %v", err)
		}
	}
}

// Count returns the number of registered integration plugins.
func (r *IntegrationRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.plugins)
}
