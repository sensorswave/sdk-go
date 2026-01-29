package sensorswave

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// IABMetaLoader metadata loader interface
type IABMetaLoader interface {
	LoadMeta() (*ABDataResp, error)
}

// HTTPSignatureMetaLoader signature authentication metadata loader - SDK default implementation
type HTTPSignatureMetaLoader struct {
	Endpoint      string
	URIPath       string // URI path for signature
	SourceToken   string
	ProjectSecret string
	HTTPClient    *httpClient
}

func (l *HTTPSignatureMetaLoader) LoadMeta() (*ABDataResp, error) {
	headers := make(map[string]string)
	headers["Content-Type"] = "application/json"
	headers[HeaderSourceToken] = l.SourceToken
	headers["X-SDK"] = sdkType
	headers["X-SDK-Version"] = strings.TrimPrefix(version, "v")

	uriPath := l.URIPath
	requestURL := strings.TrimRight(l.Endpoint, "/") + uriPath

	// Use signature authentication
	// default empty body for GET
	auth := SignRequest("GET", uriPath, "", headers, nil, l.SourceToken, l.ProjectSecret)
	headers["Authorization"] = auth

	// HTTP request
	opts := newRequestOpts().WithMethod("GET").WithURL(requestURL).WithHeaders(headers).
		WithRetry(2)

	respbody, httpcode, err := l.HTTPClient.Do(context.Background(), opts)
	if err != nil || httpcode != http.StatusOK {
		return nil, fmt.Errorf("load meta failed: %v, httpcode: %d", err, httpcode)
	}

	// Parse response
	abconf := httpResponseABLoadRemoteMeta{}
	if err = json.Unmarshal(respbody, &abconf); err != nil {
		return nil, fmt.Errorf("unmarshal failed: %v", err)
	}

	return &abconf.Data, nil
}
