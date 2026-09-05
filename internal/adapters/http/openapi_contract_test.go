package http

import (
	"context"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestOpenAPIContractIsValid(t *testing.T) {
	t.Parallel()
	document, err := openapi3.NewLoader().LoadFromFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := document.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if document.Components == nil || document.Components.Schemas["AdminAPIResult"] == nil {
		t.Fatal("components.schemas.AdminAPIResult is required")
	}
	for route, item := range document.Paths.Map() {
		for method, operation := range item.Operations() {
			if operation.OperationID == "" {
				t.Errorf("%s %s is missing operationId", method, route)
			}
		}
	}
}
