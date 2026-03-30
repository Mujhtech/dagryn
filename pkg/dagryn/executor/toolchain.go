package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mujhtech/dagryn/pkg/dagryn/toolchain"
)

// ensureToolchainsForCommand auto-bootstraps commonly required runtimes when
// task commands reference them and they are not available on PATH.
func ensureToolchainsForCommand(ctx context.Context, command string, env map[string]string, pluginPaths []string) (map[string]string, error) {
	if os.Getenv("DAGRYN_DISABLE_AUTO_TOOLCHAIN") == "1" {
		return nil, nil
	}

	required := toolchain.DetectRuntimes(command)
	if len(required) == 0 {
		return nil, nil
	}

	basePath := env["PATH"]
	if basePath == "" {
		basePath = os.Getenv("PATH")
	}

	lookupPath := basePath
	if len(pluginPaths) > 0 {
		prefix := strings.Join(pluginPaths, string(filepath.ListSeparator))
		if lookupPath != "" {
			lookupPath = prefix + string(filepath.ListSeparator) + lookupPath
		} else {
			lookupPath = prefix
		}
	}

	effectivePath := basePath
	out := make(map[string]string)

	for _, rt := range required {
		if runtimeAvailable(rt, lookupPath) {
			continue
		}

		tc, err := toolchain.EnsureRuntimeForPath(ctx, rt, lookupPath)
		if err != nil {
			return nil, fmt.Errorf("ensure %s runtime: %w", rt, err)
		}
		if tc == nil {
			continue
		}

		if tc.BinDir != "" {
			effectivePath = toolchain.PrependPath(tc.BinDir, effectivePath)
			lookupPath = toolchain.PrependPath(tc.BinDir, lookupPath)
		}
		for k, v := range tc.Env {
			out[k] = v
		}
	}

	if effectivePath != basePath {
		out["PATH"] = effectivePath
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func runtimeAvailable(rt toolchain.Runtime, path string) bool {
	switch rt {
	case toolchain.RuntimeGo:
		return toolchain.CommandExistsInPath("go", path)
	case toolchain.RuntimeNode:
		return toolchain.CommandExistsInPath("node", path)
	default:
		return true
	}
}
