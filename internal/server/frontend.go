package server

import (
	"fmt"
	"io/fs"
	"strings"

	"github.com/gofiber/fiber/v3"
)

func registerFrontendRoutes(s *Server, distFS fs.FS) {
	if distFS != nil {
		s.app.Use("/", func(c fiber.Ctx) error {
			if c.Method() != "GET" {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
			}
			return serveFrontend(c, distFS)
		})
	} else {
		s.app.Get("/", func(ctx fiber.Ctx) error {
			return ctx.SendString(fmt.Sprintf("Hello from %s", s.app.Config().AppName))
		})
	}
}

// serveFrontend serves files from the embedded dist FS. Unrecognised paths
// fall back to index.html so the React SPA handles them.
func serveFrontend(c fiber.Ctx, distFS fs.FS) error {
	path := strings.TrimPrefix(c.Path(), "/")
	if path == "" {
		path = "index.html"
	}

	data, err := fs.ReadFile(distFS, path)
	if err != nil {
		// SPA fallback: unknown paths serve index.html
		data, err = fs.ReadFile(distFS, "index.html")
		if err != nil {
			return c.Status(fiber.StatusNotFound).SendString("Not found")
		}
		c.Type("html")
		return c.Send(data)
	}

	// Set Content-Type from extension
	ext := ""
	if dot := strings.LastIndex(path, "."); dot >= 0 {
		ext = path[dot:]
	}
	switch ext {
	case ".html":
		c.Type("html")
	case ".js", ".mjs":
		c.Set("Content-Type", "application/javascript; charset=utf-8")
	case ".css":
		c.Type("css")
	case ".ico":
		c.Type("ico")
	case ".png":
		c.Type("png")
	case ".svg":
		c.Set("Content-Type", "image/svg+xml")
	case ".json":
		c.Type("json")
	case ".woff":
		c.Set("Content-Type", "font/woff")
	case ".woff2":
		c.Set("Content-Type", "font/woff2")
	default:
		c.Set("Content-Type", "application/octet-stream")
	}

	return c.Send(data)
}
