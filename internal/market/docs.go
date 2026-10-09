package market

import _ "embed"

//go:embed openapi.json
var openAPI []byte

const swaggerHTML = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>Campus Market API</title>
<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui.css"></head>
<body><div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"></script>
<script>SwaggerUIBundle({url:'/openapi.json',dom_id:'#swagger-ui',persistAuthorization:true});</script>
</body></html>`
