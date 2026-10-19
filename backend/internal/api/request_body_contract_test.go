package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOversizedJSONRequestUsesDocumentedError(t *testing.T) {
	t.Parallel()
	handler, database := newSetupTestHandler(t)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/setup/owner",
		strings.NewReader(`{"username":"`+strings.Repeat("x", 1<<20)+`","password":"test-password"}`))
	request.Header.Set("Content-Type", "application/json")
	setSameOrigin(request)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusRequestEntityTooLarge, response.Code)
	var body errorResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	assert.Equal(t, "REQUEST_TOO_LARGE", body.Error.Code)
	assert.Equal(t, "request body must not exceed 1 MB", body.Error.Message)
	spec, err := os.ReadFile("../../../api/openapi/components/schemas/errors.yaml")
	require.NoError(t, err)
	assert.Contains(t, string(spec), "        - "+body.Error.Code+"\n", "the real refusal must be representable by the generated client")
	var owners int
	require.NoError(t, database.QueryRow("SELECT COUNT(*) FROM users").Scan(&owners))
	assert.Zero(t, owners, "an oversized body cannot create an owner")
}
