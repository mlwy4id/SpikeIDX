package http

import (
	_ "embed"
	stdhttp "net/http"

	"github.com/gin-gonic/gin"
)

//go:embed openapi.yaml
var openAPISpec []byte

const swaggerHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<title>SpikeIDX API — Swagger UI</title>
<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
<style>body { margin: 0; }</style>
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-standalone-preset.js"></script>
<script>
window.onload = function () {
  SwaggerUIBundle({
    url: "/openapi.yaml",
    dom_id: "#swagger-ui",
    presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
    layout: "StandaloneLayout",
  });
};
</script>
</body>
</html>`

func (d *Dependencies) openAPISpecHandler(c *gin.Context) {
	c.Data(stdhttp.StatusOK, "application/yaml; charset=utf-8", openAPISpec)
}

func (d *Dependencies) swaggerUIHandler(c *gin.Context) {
	c.Data(stdhttp.StatusOK, "text/html; charset=utf-8", []byte(swaggerHTML))
}

func swaggerRedirectHandler(c *gin.Context) {
	c.Redirect(stdhttp.StatusMovedPermanently, "/swagger/index.html")
}
