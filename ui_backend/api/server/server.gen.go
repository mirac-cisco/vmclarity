// Package server provides primitives to interact with the openapi HTTP API.
//
// Code generated by github.com/deepmap/oapi-codegen version v1.13.0 DO NOT EDIT.
package server

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/deepmap/oapi-codegen/pkg/runtime"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/labstack/echo/v4"
	. "github.com/openclarity/vmclarity/ui_backend/api/models"
)

// ServerInterface represents all server handlers.
type ServerInterface interface {
	// Get a list of findings impact for the dashboard.
	// (GET /dashboard/findingsImpact)
	GetDashboardFindingsImpact(ctx echo.Context) error
	// Get a list of finding trends for all finding types.
	// (GET /dashboard/findingsTrends)
	GetDashboardFindingsTrends(ctx echo.Context, params GetDashboardFindingsTrendsParams) error
	// Get a list of riskiest assets for the dashboard.
	// (GET /dashboard/riskiestAssets)
	GetDashboardRiskiestAssets(ctx echo.Context) error
	// Get a list of riskiest regions for the dashboard.
	// (GET /dashboard/riskiestRegions)
	GetDashboardRiskiestRegions(ctx echo.Context) error
}

// ServerInterfaceWrapper converts echo contexts to parameters.
type ServerInterfaceWrapper struct {
	Handler ServerInterface
}

// GetDashboardFindingsImpact converts echo context to params.
func (w *ServerInterfaceWrapper) GetDashboardFindingsImpact(ctx echo.Context) error {
	var err error

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.GetDashboardFindingsImpact(ctx)
	return err
}

// GetDashboardFindingsTrends converts echo context to params.
func (w *ServerInterfaceWrapper) GetDashboardFindingsTrends(ctx echo.Context) error {
	var err error

	// Parameter object where we will unmarshal all parameters from the context
	var params GetDashboardFindingsTrendsParams
	// ------------- Required query parameter "startTime" -------------

	err = runtime.BindQueryParameter("form", true, true, "startTime", ctx.QueryParams(), &params.StartTime)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid format for parameter startTime: %s", err))
	}

	// ------------- Required query parameter "endTime" -------------

	err = runtime.BindQueryParameter("form", true, true, "endTime", ctx.QueryParams(), &params.EndTime)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid format for parameter endTime: %s", err))
	}

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.GetDashboardFindingsTrends(ctx, params)
	return err
}

// GetDashboardRiskiestAssets converts echo context to params.
func (w *ServerInterfaceWrapper) GetDashboardRiskiestAssets(ctx echo.Context) error {
	var err error

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.GetDashboardRiskiestAssets(ctx)
	return err
}

// GetDashboardRiskiestRegions converts echo context to params.
func (w *ServerInterfaceWrapper) GetDashboardRiskiestRegions(ctx echo.Context) error {
	var err error

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.GetDashboardRiskiestRegions(ctx)
	return err
}

// This is a simple interface which specifies echo.Route addition functions which
// are present on both echo.Echo and echo.Group, since we want to allow using
// either of them for path registration
type EchoRouter interface {
	CONNECT(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	DELETE(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	GET(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	HEAD(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	OPTIONS(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	PATCH(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	POST(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	PUT(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	TRACE(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
}

// RegisterHandlers adds each server route to the EchoRouter.
func RegisterHandlers(router EchoRouter, si ServerInterface) {
	RegisterHandlersWithBaseURL(router, si, "")
}

// Registers handlers, and prepends BaseURL to the paths, so that the paths
// can be served under a prefix.
func RegisterHandlersWithBaseURL(router EchoRouter, si ServerInterface, baseURL string) {

	wrapper := ServerInterfaceWrapper{
		Handler: si,
	}

	router.GET(baseURL+"/dashboard/findingsImpact", wrapper.GetDashboardFindingsImpact)
	router.GET(baseURL+"/dashboard/findingsTrends", wrapper.GetDashboardFindingsTrends)
	router.GET(baseURL+"/dashboard/riskiestAssets", wrapper.GetDashboardRiskiestAssets)
	router.GET(baseURL+"/dashboard/riskiestRegions", wrapper.GetDashboardRiskiestRegions)

}

// Base64 encoded, gzipped, json marshaled Swagger object
var swaggerSpec = []string{

	"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",
}

// GetSwagger returns the content of the embedded swagger specification file
// or error if failed to decode
func decodeSpec() ([]byte, error) {
	zipped, err := base64.StdEncoding.DecodeString(strings.Join(swaggerSpec, ""))
	if err != nil {
		return nil, fmt.Errorf("error base64 decoding spec: %s", err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		return nil, fmt.Errorf("error decompressing spec: %s", err)
	}
	var buf bytes.Buffer
	_, err = buf.ReadFrom(zr)
	if err != nil {
		return nil, fmt.Errorf("error decompressing spec: %s", err)
	}

	return buf.Bytes(), nil
}

var rawSpec = decodeSpecCached()

// a naive cached of a decoded swagger spec
func decodeSpecCached() func() ([]byte, error) {
	data, err := decodeSpec()
	return func() ([]byte, error) {
		return data, err
	}
}

// Constructs a synthetic filesystem for resolving external references when loading openapi specifications.
func PathToRawSpec(pathToFile string) map[string]func() ([]byte, error) {
	var res = make(map[string]func() ([]byte, error))
	if len(pathToFile) > 0 {
		res[pathToFile] = rawSpec
	}

	return res
}

// GetSwagger returns the Swagger specification corresponding to the generated code
// in this file. The external references of Swagger specification are resolved.
// The logic of resolving external references is tightly connected to "import-mapping" feature.
// Externally referenced files must be embedded in the corresponding golang packages.
// Urls can be supported but this task was out of the scope.
func GetSwagger() (swagger *openapi3.T, err error) {
	var resolvePath = PathToRawSpec("")

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	loader.ReadFromURIFunc = func(loader *openapi3.Loader, url *url.URL) ([]byte, error) {
		var pathToFile = url.String()
		pathToFile = path.Clean(pathToFile)
		getSpec, ok := resolvePath[pathToFile]
		if !ok {
			err1 := fmt.Errorf("path not found: %s", pathToFile)
			return nil, err1
		}
		return getSpec()
	}
	var specData []byte
	specData, err = rawSpec()
	if err != nil {
		return
	}
	swagger, err = loader.LoadFromData(specData)
	if err != nil {
		return
	}
	return
}
