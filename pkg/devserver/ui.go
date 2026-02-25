package devserver

import (
	"context"
	"embed"
	"mime"
	"path"
	"strings"
)

//go:embed all:static
var static embed.FS

func init() {
	//
	// Fix invalid mime type errors when loading JS from our assets on windows
	_ = mime.AddExtensionType(".js", "application/javascript")
}

// StaticFS returns the embedded filesystem containing the built UI assets.
func StaticFS() embed.FS {
	return static
}

// Serve implements SPA routing for Tanstack assets, exported for use by
// the ui-only command. It returns the file bytes for the given request path,
// falling back to _shell.html for client-side routing.
func Serve(requestPath string) []byte {
	return serve(context.Background(), requestPath)
}

// serve implements SPA routing for Tanstack assets:
// - Serves static files from static/client if they exist
// - Falls back to _shell.html for all other routes (client-side routing)
func serve(ctx context.Context, requestPath string) []byte {
	//
	// Try to serve the file directly from static/client
	filePath := path.Join("static/client", requestPath)

	if byt, err := static.ReadFile(filePath); err == nil {
		return byt
	}

	//
	// If the path has a file extension, it was likely a missing asset
	// Don't fallback to shell in this case
	if strings.Contains(path.Base(requestPath), ".") {
		return nil
	}

	//
	// Fall back to _shell.html for client-side routing
	byt, _ := static.ReadFile("static/client/_shell.html")
	return byt
}
