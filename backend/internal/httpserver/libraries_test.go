package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/yaremam/opusflow/backend/internal/library"
)

func TestCreateLibraryEndpoint(t *testing.T) {
	svc := testService(t)
	root := t.TempDir()

	body := `{"name":"Main Collection","rootPath":"` + jsonEscape(root) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/libraries", strings.NewReader(body))
	rec := httptest.NewRecorder()
	New("", "", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var got library.Library
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v, body = %s", err, rec.Body.String())
	}
	if got.Name != "Main Collection" || got.RootPath != root {
		t.Fatalf("library = %+v", got)
	}
}

func TestCreateLibraryEndpointRejectsNonexistentPath(t *testing.T) {
	svc := testService(t)

	body := `{"name":"Main Collection","rootPath":"/does/not/exist"}`
	req := httptest.NewRequest(http.MethodPost, "/api/libraries", strings.NewReader(body))
	rec := httptest.NewRecorder()
	New("", "", "", "", svc).ServeHTTP(rec, req)

	if rec.Code == http.StatusCreated {
		t.Fatalf("status = %d, want an error status for a nonexistent path", rec.Code)
	}
}

// TestCreateLibraryEndpointRejectsPathOutsideConfiguredRoot mirrors
// TestImportBrowseRejectsPathOutsideConfiguredRoot and
// TestCreateFolderEndpointRejectsPathOutsideConfiguredRoot — the same
// DATA_DIR scoping those two already cover at the HTTP layer, but nothing
// exercised it for library creation specifically. Uses lazyService rather
// than testService since library.Service.CreateLibrary rejects an
// out-of-root path via WithinRoot before ever touching the store.
func TestCreateLibraryEndpointRejectsPathOutsideConfiguredRoot(t *testing.T) {
	svc := lazyService(t)
	svc.SetBrowseRoot(t.TempDir())

	outside := t.TempDir() // a sibling temp dir, not under the configured root
	body := `{"name":"Main Collection","rootPath":"` + jsonEscape(outside) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/libraries", strings.NewReader(body))
	rec := httptest.NewRecorder()
	New("", "", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestCreateLibraryEndpointRequiresName(t *testing.T) {
	svc := testService(t)

	body := `{"rootPath":"` + jsonEscape(t.TempDir()) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/libraries", strings.NewReader(body))
	rec := httptest.NewRecorder()
	New("", "", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestListLibrariesEndpoint(t *testing.T) {
	svc := testService(t)
	if _, err := svc.CreateLibrary(t.Context(), "Main Collection", t.TempDir()); err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	if _, err := svc.CreateLibrary(t.Context(), "Kids Music", t.TempDir()); err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/libraries", nil)
	rec := httptest.NewRecorder()
	New("", "", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var got []library.Library
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v, body = %s", err, rec.Body.String())
	}
	if len(got) != 2 {
		t.Fatalf("libraries = %+v, want 2", got)
	}
}

func TestDeleteLibraryEndpointRequiresDeleteFilesParam(t *testing.T) {
	svc := testService(t)
	lib, err := svc.CreateLibrary(t.Context(), "Main Collection", t.TempDir())
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/libraries/"+strconv.FormatInt(lib.ID, 10), nil)
	rec := httptest.NewRecorder()
	New("", "", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestDeleteLibraryEndpointRemovesLibrary(t *testing.T) {
	svc := testService(t)
	lib, err := svc.CreateLibrary(t.Context(), "Main Collection", t.TempDir())
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/libraries/"+strconv.FormatInt(lib.ID, 10)+"?deleteFiles=false", nil)
	rec := httptest.NewRecorder()
	New("", "", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}

	libs, err := svc.ListLibraries(t.Context())
	if err != nil {
		t.Fatalf("ListLibraries: %v", err)
	}
	if len(libs) != 0 {
		t.Fatalf("libraries after delete = %+v, want none", libs)
	}
}

func TestDeleteLibraryEndpointNotFound(t *testing.T) {
	svc := testService(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/libraries/999999?deleteFiles=false", nil)
	rec := httptest.NewRecorder()
	New("", "", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}
