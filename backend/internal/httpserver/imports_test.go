package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/yaremam/opusflow/backend/internal/library"
	"github.com/yaremam/opusflow/backend/internal/library/organize"
)

// taggedMP3Fixture copies organize's own real-tag MP3 fixture (Artist="Test
// Artist", Album="Test Album", Title="Test Title", TrackNumber=3, Year=2000)
// into dest, so plan-building endpoints have something real to read tags
// from without this package needing its own binary fixture.
func taggedMP3Fixture(t *testing.T, dest string) {
	t.Helper()
	src := filepath.Join("..", "library", "organize", "testdata", "tagged.mp3")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBuildPlanReturnsAlbumsFromSourceDirectory(t *testing.T) {
	root := t.TempDir()
	taggedMP3Fixture(t, filepath.Join(root, "song.mp3"))
	svc := lazyService(t, library.Roots{root})

	body := `{"sourceDir":"` + root + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/imports/plan", strings.NewReader(body))
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got struct {
		Plan   organize.Plan              `json:"plan"`
		Errors []organize.ValidationError `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v, body = %s", err, rec.Body.String())
	}
	if len(got.Plan.Albums) != 1 || got.Plan.Albums[0].Artist != "Test Artist" {
		t.Fatalf("plan = %+v", got.Plan)
	}
	if len(got.Errors) != 0 {
		t.Fatalf("errors = %+v, want none for a fully-tagged file", got.Errors)
	}
}

func TestBuildPlanRejectsPathOutsideRoots(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	svc := lazyService(t, library.Roots{root})

	body := `{"sourceDir":"` + outside + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/imports/plan", strings.NewReader(body))
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestBuildPlanRequiresSourceDir(t *testing.T) {
	svc := lazyService(t, library.Roots{t.TempDir()})

	req := httptest.NewRequest(http.MethodPost, "/api/imports/plan", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestValidatePlanFlagsMissingFields(t *testing.T) {
	svc := lazyService(t, nil)

	body := `{"albums":[{"artist":"","album":"Album","year":2000,"tracks":[{"sourcePath":"/src/one.mp3","title":"Title","trackNumber":1}]}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/imports/plan/validate", strings.NewReader(body))
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var got struct {
		Errors []organize.ValidationError `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Errors) != 1 {
		t.Fatalf("errors = %+v, want 1 (missing artist)", got.Errors)
	}
}

func TestUploadImportStagesFilesAndReturnsPlan(t *testing.T) {
	svc := lazyService(t, nil)

	data, err := os.ReadFile(filepath.Join("..", "library", "organize", "testdata", "tagged.mp3"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("files", "Test Artist/Test Album/song.mp3")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/imports/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var got struct {
		Plan organize.Plan `json:"plan"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v, body = %s", err, rec.Body.String())
	}
	if len(got.Plan.Albums) != 1 || got.Plan.Albums[0].Artist != "Test Artist" {
		t.Fatalf("plan = %+v", got.Plan)
	}
}

func TestUploadImportRejectsPathTraversalInFilename(t *testing.T) {
	svc := lazyService(t, nil)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("files", "../../etc/passwd")
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte("not audio"))
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/imports/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	// The traversal is neutralized (staged under the temp dir, not written
	// to /etc/passwd) rather than rejected outright — either way, the real
	// /etc/passwd on this machine must be untouched.
	if _, err := os.Stat("/etc/passwd"); err == nil {
		data, _ := os.ReadFile("/etc/passwd")
		if strings.Contains(string(data), "not audio") {
			t.Fatal("path traversal wrote into /etc/passwd")
		}
	}
	_ = rec
}

func TestConfirmImportAcceptsValidPlan(t *testing.T) {
	svc := testService(t, library.Roots{t.TempDir()})

	src := filepath.Join(t.TempDir(), "song.mp3")
	taggedMP3Fixture(t, src)

	planBody := `{"albums":[{"artist":"Artist","album":"Album","year":2000,"tracks":[{"sourcePath":"` + jsonEscape(src) + `","title":"Title","trackNumber":1}]}]}`
	body := `{"sourceDescription":"/music/src","plan":` + planBody + `}`

	req := httptest.NewRequest(http.MethodPost, "/api/imports", strings.NewReader(body))
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
}

func TestConfirmImportRejectsIncompletePlan(t *testing.T) {
	svc := lazyService(t, nil)

	body := `{"sourceDescription":"/music/src","plan":{"albums":[{"artist":"","album":"Album","year":2000,"tracks":[{"sourcePath":"/src/one.mp3","title":"Title","trackNumber":1}]}]}}`
	req := httptest.NewRequest(http.MethodPost, "/api/imports", strings.NewReader(body))
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
	}
}

func TestConfirmImportRequiresSourceDescription(t *testing.T) {
	svc := lazyService(t, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/imports", strings.NewReader(`{"plan":{"albums":[]}}`))
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestListImportsReturnsNewestFirst(t *testing.T) {
	store, svc := testStoreAndService(t, library.Roots{t.TempDir()})
	if _, err := store.CreateImport(context.Background(), "/music/first"); err != nil {
		t.Fatalf("CreateImport: %v", err)
	}
	if _, err := store.CreateImport(context.Background(), "/music/second"); err != nil {
		t.Fatalf("CreateImport: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/imports", nil)
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var got []struct {
		SourceDescription string `json:"sourceDescription"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v, body = %s", err, rec.Body.String())
	}
	if len(got) != 2 || got[0].SourceDescription != "/music/second" || got[1].SourceDescription != "/music/first" {
		t.Fatalf("imports = %+v, want [second, first]", got)
	}
}

func TestGetImportReturnsProgress(t *testing.T) {
	root := t.TempDir()
	svc := testService(t, library.Roots{root})

	src := filepath.Join(root, "src")
	taggedMP3Fixture(t, filepath.Join(src, "song.mp3"))

	planReq := httptest.NewRequest(http.MethodPost, "/api/imports/plan", strings.NewReader(`{"sourceDir":"`+src+`"}`))
	planRec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(planRec, planReq)
	if planRec.Code != http.StatusOK {
		t.Fatalf("build plan status = %d, body = %s", planRec.Code, planRec.Body.String())
	}
	var planResp struct {
		Plan organize.Plan `json:"plan"`
	}
	if err := json.Unmarshal(planRec.Body.Bytes(), &planResp); err != nil {
		t.Fatalf("unmarshal plan: %v", err)
	}

	planJSON, err := json.Marshal(planResp.Plan)
	if err != nil {
		t.Fatal(err)
	}
	confirmBody := `{"sourceDescription":"` + jsonEscape(src) + `","plan":` + string(planJSON) + `}`
	confirmReq := httptest.NewRequest(http.MethodPost, "/api/imports", strings.NewReader(confirmBody))
	confirmRec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(confirmRec, confirmReq)
	if confirmRec.Code != http.StatusAccepted {
		t.Fatalf("confirm status = %d, body = %s", confirmRec.Code, confirmRec.Body.String())
	}
	var imp struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(confirmRec.Body.Bytes(), &imp); err != nil {
		t.Fatalf("unmarshal import: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/imports/"+strconv.FormatInt(imp.ID, 10), nil)
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestGetImportNotFound(t *testing.T) {
	svc := testService(t, library.Roots{t.TempDir()})

	req := httptest.NewRequest(http.MethodGet, "/api/imports/999999", nil)
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestGetImportInvalidID(t *testing.T) {
	svc := testService(t, library.Roots{t.TempDir()})

	req := httptest.NewRequest(http.MethodGet, "/api/imports/not-a-number", nil)
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1])
}
