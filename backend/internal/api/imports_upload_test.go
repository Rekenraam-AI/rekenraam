package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// serial: TMPDIR isolates multipart spill files so cleanup is observable even
// when handlers are called directly (without net/http's server cleanup).
func TestImportUploadFileSizeBoundary(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TMPDIR", tempDir)
	for _, format := range []string{"qif", "csv"} {
		t.Run(format, func(t *testing.T) {
			handler, database := newSetupTestHandler(t)
			cookie, csrf, _, _, _ := bootstrapImportAPITest(t, handler)
			payload := importUploadBoundaryPayload(format)
			for _, size := range []int{maxImportUploadSize, maxImportUploadSize + 1} {
				for _, unknownLength := range []bool{false, true} {
					name := "supported_size"
					if size > maxImportUploadSize {
						name = "one_byte_over_parseable_prefix"
					}
					if unknownLength {
						name += "_unknown_length"
					}
					t.Run(name, func(t *testing.T) {
						before := importUploadDurableCounts(t, database)
						path := "/api/v1/imports"
						if format == "csv" {
							path += "/analyze"
						}
						req := importUploadRequest(t, path, "history."+format, bytes.NewReader(payload[:size]))
						if unknownLength {
							req.ContentLength = -1
						}
						req.AddCookie(cookie)
						req.Header.Set(csrfTokenHeader, csrf)
						setSameOrigin(req)
						res := httptest.NewRecorder()
						handler.ServeHTTP(res, req)
						if size > maxImportUploadSize {
							require.Equal(t, http.StatusRequestEntityTooLarge, res.Code, res.Body.String())
							assert.JSONEq(t, `{"error":{"code":"IMPORT_UPLOAD_TOO_LARGE","message":"import upload exceeds resource limits (50 MiB file, 51 MiB request)"}}`, res.Body.String())
							assert.Equal(t, before, importUploadDurableCounts(t, database))
						} else if format == "qif" {
							require.Equal(t, http.StatusCreated, res.Code, res.Body.String())
							var started startImportResponse
							require.NoError(t, json.Unmarshal(res.Body.Bytes(), &started))
							require.Len(t, started.Rows, 1, "the entire supported prefix must be independently parseable")
						} else {
							require.Equal(t, http.StatusOK, res.Code, res.Body.String())
							assert.Contains(t, res.Body.String(), `"headers":["date","memo","amount"]`)
							assert.Equal(t, before, importUploadDurableCounts(t, database))
						}
						assertImportMultipartCleaned(t, req, tempDir)
					})
				}
			}
		})
	}
}

// serial: TMPDIR isolates multipart spill files for direct-handler assertions.
func TestImportUploadMultipartResourceLimits(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TMPDIR", tempDir)
	handler, database := newSetupTestHandler(t)
	cookie, csrf, _, _, _ := bootstrapImportAPITest(t, handler)
	for _, path := range []string{"/api/v1/imports", "/api/v1/imports/analyze"} {
		for _, scenario := range []struct {
			name       string
			wantStatus int
		}{
			{"request_over_limit_known_length", http.StatusRequestEntityTooLarge},
			{"request_over_limit_unknown_length", http.StatusRequestEntityTooLarge},
			{"excessive_form_field_after_spill", http.StatusRequestEntityTooLarge},
			{"too_many_parts_after_spill", http.StatusRequestEntityTooLarge},
			{"too_many_headers_after_spill", http.StatusRequestEntityTooLarge},
			{"malformed_form_after_spill", http.StatusBadRequest},
			{"missing_file_after_spill", http.StatusBadRequest},
			{"adapter_failure_after_spill", http.StatusBadRequest},
			{"epilogue_over_limit_unknown_length", http.StatusRequestEntityTooLarge},
		} {
			t.Run(path+"/"+scenario.name, func(t *testing.T) {
				before := importUploadDurableCounts(t, database)
				var body bytes.Buffer
				writer := multipart.NewWriter(&body)
				field := "file"
				if scenario.name == "missing_file_after_spill" {
					field = "other"
				}
				part, err := writer.CreateFormFile(field, "invalid.csv")
				require.NoError(t, err)
				contentSize := 2 << 20
				if strings.HasPrefix(scenario.name, "request_over_limit") {
					contentSize = maxImportRequestSize + 1
				}
				_, err = part.Write(bytes.Repeat([]byte{'x'}, contentSize))
				require.NoError(t, err)
				switch scenario.name {
				case "excessive_form_field_after_spill":
					require.NoError(t, writer.WriteField("metadata", strings.Repeat("x", 12<<20)))
				case "too_many_parts_after_spill":
					for range 1100 {
						require.NoError(t, writer.WriteField("extra", ""))
					}
				case "too_many_headers_after_spill":
					header := make(textproto.MIMEHeader)
					header.Set("Content-Disposition", `form-data; name="extra"`)
					for range 11000 {
						header.Add("X-Extra", "x")
					}
					_, err := writer.CreatePart(header)
					require.NoError(t, err)
				}
				if scenario.name != "malformed_form_after_spill" {
					require.NoError(t, writer.Close())
				}
				if scenario.name == "epilogue_over_limit_unknown_length" {
					_, err = body.Write(bytes.Repeat([]byte{'x'}, maxImportRequestSize))
					require.NoError(t, err)
				}
				req := httptest.NewRequest(http.MethodPost, path, &body)
				if strings.Contains(scenario.name, "unknown_length") {
					req.ContentLength = -1
				}
				counter := &importUploadCountingBody{ReadCloser: req.Body}
				req.Body = counter
				req.Header.Set("Content-Type", writer.FormDataContentType())
				req.Header.Set(csrfTokenHeader, csrf)
				setSameOrigin(req)
				req.AddCookie(cookie)
				res := httptest.NewRecorder()
				handler.ServeHTTP(res, req)
				require.Equal(t, scenario.wantStatus, res.Code, res.Body.String())
				if scenario.wantStatus == http.StatusRequestEntityTooLarge {
					assert.Contains(t, res.Body.String(), `"code":"IMPORT_UPLOAD_TOO_LARGE"`)
				} else {
					assert.Contains(t, res.Body.String(), `"code":"VALIDATION_FAILED"`)
				}
				assert.LessOrEqual(t, counter.bytesRead, int64(maxImportRequestSize+1), "the request body must be bounded even without Content-Length")
				if scenario.name == "request_over_limit_known_length" {
					assert.Zero(t, counter.bytesRead, "reject known oversize before buffering")
				}
				assert.Equal(t, before, importUploadDurableCounts(t, database))
				assertImportMultipartCleaned(t, req, tempDir)
			})
		}
	}
}

type importUploadCountingBody struct {
	io.ReadCloser
	bytesRead int64
}

func (r *importUploadCountingBody) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	r.bytesRead += int64(n)
	return n, err
}

func assertImportMultipartCleaned(t *testing.T, req *http.Request, tempDir string) {
	t.Helper()
	spills, err := filepath.Glob(filepath.Join(tempDir, "multipart-*"))
	require.NoError(t, err)
	assert.Empty(t, spills, "multipart temporary files must be removed")
	if req.MultipartForm != nil {
		for _, files := range req.MultipartForm.File {
			for _, header := range files {
				if header.Size <= maxImportMultipartMemory {
					continue
				}
				file, err := header.Open()
				if file != nil {
					require.NoError(t, file.Close())
				}
				assert.ErrorIs(t, err, os.ErrNotExist, "prove a spilled file was removed")
			}
		}
	}
}

// The exact 50 MiB QIF prefix finishes a valid record; ignored option lines
// provide padding without creating millions of staged rows. CSV analysis sees
// a valid header followed by blank lines. The extra byte must never be dropped.
func importUploadBoundaryPayload(format string) []byte {
	payload := bytes.Repeat([]byte{'\n'}, maxImportUploadSize+1)
	if format == "csv" {
		copy(payload, "date,memo,amount\n")
		return payload
	}
	copy(payload, "!Type:Bank\n")
	padding := "!Option:" + strings.Repeat("x", 4087) + "\n"
	record := "D10/14/2026\nT-1.00\nPTest shop\n^\n"
	end := maxImportUploadSize - len(record)
	for offset := len("!Type:Bank\n"); offset+len(padding) <= end; offset += len(padding) {
		copy(payload[offset:], padding)
	}
	copy(payload[end:], record)
	return payload
}

func importUploadRequest(t *testing.T, path, filename string, content io.Reader) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("text_encoding", "utf-8"))
	require.NoError(t, writer.WriteField("delimiter", "comma"))
	part, err := writer.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = io.Copy(part, content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func importUploadDurableCounts(t *testing.T, database *sql.DB) map[string]int {
	t.Helper()
	counts := make(map[string]int)
	for _, table := range []string{"import_batches", "import_batch_events", "import_staged_rows", "import_commit_identities", "transactions", "transaction_versions", "posting_versions", "investment_operations", "investment_lots", "audit_events", "background_work_items"} {
		var count int
		require.NoError(t, database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM "+table).Scan(&count))
		counts[table] = count
	}
	return counts
}
