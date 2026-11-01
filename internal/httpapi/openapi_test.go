package httpapi_test

import (
	"os"
	"strings"
	"testing"
)

// The OpenAPI file must document exactly the routes the server registers.
func TestRoutesMatchOpenAPI(t *testing.T) {
	raw, err := os.ReadFile("../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	methods := map[string]bool{"get": true, "post": true, "put": true, "patch": true, "delete": true}
	documented := map[string]bool{}
	path := ""
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "  /") && strings.HasSuffix(line, ":") {
			path = strings.TrimSuffix(strings.TrimSpace(line), ":")
			continue
		}
		if path != "" && strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "     ") && strings.HasSuffix(line, ":") {
			m := strings.TrimSuffix(strings.TrimSpace(line), ":")
			if methods[m] {
				documented[strings.ToUpper(m)+" "+path] = true
			}
		}
	}
	e := newEnv(t)
	routes := map[string]bool{}
	for _, r := range e.srv.Routes() {
		routes[r] = true
		if !documented[r] {
			t.Errorf("route not documented in openapi.yaml: %s", r)
		}
	}
	for d := range documented {
		if !routes[d] {
			t.Errorf("openapi.yaml documents a route that does not exist: %s", d)
		}
	}
}
