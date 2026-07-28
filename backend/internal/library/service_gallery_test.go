package library

import (
	"os"
	"testing"

	"github.com/yaremam/opusflow/backend/internal/library/enrich"
)

func TestServiceListArtistPhotosDelegates(t *testing.T) {
	store := newCatalogCapturingStore()
	svc := NewService(store, newRecordingCopier())
	svc.SetImages(enrich.NewImageStore(t.TempDir()))

	if _, err := svc.UploadArtistArt(ctx(), 1, onePixelPNG()); err != nil {
		t.Fatalf("UploadArtistArt: %v", err)
	}

	photos, err := svc.ListArtistPhotos(ctx(), 1)
	if err != nil {
		t.Fatalf("ListArtistPhotos: %v", err)
	}
	if len(photos) != 1 {
		t.Fatalf("len(photos) = %d, want 1", len(photos))
	}
}

func TestServiceSetArtistPrimaryPhotoDelegates(t *testing.T) {
	store := newCatalogCapturingStore()
	svc := NewService(store, newRecordingCopier())
	svc.SetImages(enrich.NewImageStore(t.TempDir()))

	if _, err := svc.UploadArtistArt(ctx(), 1, onePixelPNG()); err != nil {
		t.Fatalf("UploadArtistArt: %v", err)
	}
	second, _ := store.AddArtistPhoto(ctx(), 1, "/b/thumb.jpg", "/b/full.jpg", "upload", "hash-b")

	if err := svc.SetArtistPrimaryPhoto(ctx(), 1, second.ID); err != nil {
		t.Fatalf("SetArtistPrimaryPhoto: %v", err)
	}
	photos, _ := svc.ListArtistPhotos(ctx(), 1)
	for _, p := range photos {
		if p.ID == second.ID && !p.IsPrimary {
			t.Fatalf("photo %d not marked primary after SetArtistPrimaryPhoto", p.ID)
		}
	}
}

func TestServiceSetArtistPrimaryPhotoPropagatesNotFound(t *testing.T) {
	store := newFakeImportStore()
	svc := NewService(store, newRecordingCopier())

	if err := svc.SetArtistPrimaryPhoto(ctx(), 1, 999); err != ErrArtistPhotoNotFound {
		t.Fatalf("SetArtistPrimaryPhoto error = %v, want ErrArtistPhotoNotFound", err)
	}
}

func TestServiceSetArtistBannerPhotoDelegates(t *testing.T) {
	store := newCatalogCapturingStore()
	svc := NewService(store, newRecordingCopier())
	svc.SetImages(enrich.NewImageStore(t.TempDir()))

	if _, err := svc.UploadArtistArt(ctx(), 1, onePixelPNG()); err != nil {
		t.Fatalf("UploadArtistArt: %v", err)
	}
	second, _ := store.AddArtistPhoto(ctx(), 1, "/b/thumb.jpg", "/b/full.jpg", "upload", "hash-b")

	if err := svc.SetArtistBannerPhoto(ctx(), 1, second.ID); err != nil {
		t.Fatalf("SetArtistBannerPhoto: %v", err)
	}
	photos, _ := svc.ListArtistPhotos(ctx(), 1)
	for _, p := range photos {
		if p.ID == second.ID && !p.IsBanner {
			t.Fatalf("photo %d not marked banner after SetArtistBannerPhoto", p.ID)
		}
	}
}

func TestServiceSetArtistBannerPhotoPropagatesNotFound(t *testing.T) {
	store := newFakeImportStore()
	svc := NewService(store, newRecordingCopier())

	if err := svc.SetArtistBannerPhoto(ctx(), 1, 999); err != ErrArtistPhotoNotFound {
		t.Fatalf("SetArtistBannerPhoto error = %v, want ErrArtistPhotoNotFound", err)
	}
}

func TestServiceDeleteArtistPhotoKeepsFileByDefault(t *testing.T) {
	store := newCatalogCapturingStore()
	svc := NewService(store, newRecordingCopier())
	dir := t.TempDir()
	svc.SetImages(enrich.NewImageStore(dir))

	if _, err := svc.UploadArtistArt(ctx(), 1, onePixelPNG()); err != nil {
		t.Fatalf("UploadArtistArt: %v", err)
	}
	photo := store.artistPhotos[1][0]
	diskPath := dir + photo.ThumbURL[len("/artwork"):]

	if err := svc.DeleteArtistPhoto(ctx(), 1, photo.ID, false); err != nil {
		t.Fatalf("DeleteArtistPhoto: %v", err)
	}
	if len(store.artistPhotos[1]) != 0 {
		t.Fatalf("artistPhotos[1] = %+v, want empty after delete", store.artistPhotos[1])
	}
	if _, err := os.Stat(diskPath); err != nil {
		t.Fatalf("expected file %q to survive a keep-file delete, stat error: %v", diskPath, err)
	}
}

func TestServiceDeleteArtistPhotoRemovesFileWhenRequested(t *testing.T) {
	store := newCatalogCapturingStore()
	svc := NewService(store, newRecordingCopier())
	dir := t.TempDir()
	svc.SetImages(enrich.NewImageStore(dir))

	if _, err := svc.UploadArtistArt(ctx(), 1, onePixelPNG()); err != nil {
		t.Fatalf("UploadArtistArt: %v", err)
	}
	photo := store.artistPhotos[1][0]
	diskPath := dir + photo.ThumbURL[len("/artwork"):]

	if err := svc.DeleteArtistPhoto(ctx(), 1, photo.ID, true); err != nil {
		t.Fatalf("DeleteArtistPhoto: %v", err)
	}
	if _, err := os.Stat(diskPath); !os.IsNotExist(err) {
		t.Fatalf("expected file %q to be removed, stat error: %v", diskPath, err)
	}
}

func TestServiceDeleteArtistPhotoPropagatesNotFound(t *testing.T) {
	store := newFakeImportStore()
	svc := NewService(store, newRecordingCopier())

	if err := svc.DeleteArtistPhoto(ctx(), 1, 999, false); err != ErrArtistPhotoNotFound {
		t.Fatalf("DeleteArtistPhoto error = %v, want ErrArtistPhotoNotFound", err)
	}
}

func TestServiceListAlbumCoversDelegates(t *testing.T) {
	store := newCatalogCapturingStore()
	svc := NewService(store, newRecordingCopier())
	svc.SetImages(enrich.NewImageStore(t.TempDir()))

	if _, err := svc.UploadAlbumArt(ctx(), 7, onePixelPNG()); err != nil {
		t.Fatalf("UploadAlbumArt: %v", err)
	}

	covers, err := svc.ListAlbumCovers(ctx(), 7)
	if err != nil {
		t.Fatalf("ListAlbumCovers: %v", err)
	}
	if len(covers) != 1 {
		t.Fatalf("len(covers) = %d, want 1", len(covers))
	}
}

func TestServiceSetAlbumPrimaryCoverDelegates(t *testing.T) {
	store := newCatalogCapturingStore()
	svc := NewService(store, newRecordingCopier())
	svc.SetImages(enrich.NewImageStore(t.TempDir()))

	if _, err := svc.UploadAlbumArt(ctx(), 7, onePixelPNG()); err != nil {
		t.Fatalf("UploadAlbumArt: %v", err)
	}
	second, _ := store.AddAlbumCover(ctx(), 7, "/b/thumb.jpg", "/b/full.jpg", "upload", "", "hash-b")

	if err := svc.SetAlbumPrimaryCover(ctx(), 7, second.ID); err != nil {
		t.Fatalf("SetAlbumPrimaryCover: %v", err)
	}
	covers, _ := svc.ListAlbumCovers(ctx(), 7)
	for _, c := range covers {
		if c.ID == second.ID && !c.IsPrimary {
			t.Fatalf("cover %d not marked primary after SetAlbumPrimaryCover", c.ID)
		}
	}
}

func TestServiceSetAlbumBannerCoverDelegates(t *testing.T) {
	store := newCatalogCapturingStore()
	svc := NewService(store, newRecordingCopier())
	svc.SetImages(enrich.NewImageStore(t.TempDir()))

	if _, err := svc.UploadAlbumArt(ctx(), 7, onePixelPNG()); err != nil {
		t.Fatalf("UploadAlbumArt: %v", err)
	}
	second, _ := store.AddAlbumCover(ctx(), 7, "/b/thumb.jpg", "/b/full.jpg", "upload", "", "hash-b")

	if err := svc.SetAlbumBannerCover(ctx(), 7, second.ID); err != nil {
		t.Fatalf("SetAlbumBannerCover: %v", err)
	}
	covers, _ := svc.ListAlbumCovers(ctx(), 7)
	for _, c := range covers {
		if c.ID == second.ID && !c.IsBanner {
			t.Fatalf("cover %d not marked banner after SetAlbumBannerCover", c.ID)
		}
	}
}

func TestServiceSetAlbumBannerCoverPropagatesNotFound(t *testing.T) {
	store := newFakeImportStore()
	svc := NewService(store, newRecordingCopier())

	if err := svc.SetAlbumBannerCover(ctx(), 7, 999); err != ErrAlbumCoverNotFound {
		t.Fatalf("SetAlbumBannerCover error = %v, want ErrAlbumCoverNotFound", err)
	}
}

func TestServiceDeleteAlbumCoverRemovesFileWhenRequested(t *testing.T) {
	store := newCatalogCapturingStore()
	svc := NewService(store, newRecordingCopier())
	dir := t.TempDir()
	svc.SetImages(enrich.NewImageStore(dir))

	if _, err := svc.UploadAlbumArt(ctx(), 7, onePixelPNG()); err != nil {
		t.Fatalf("UploadAlbumArt: %v", err)
	}
	cover := store.albumCovers[7][0]
	diskPath := dir + cover.ThumbURL[len("/artwork"):]

	if err := svc.DeleteAlbumCover(ctx(), 7, cover.ID, true); err != nil {
		t.Fatalf("DeleteAlbumCover: %v", err)
	}
	if _, err := os.Stat(diskPath); !os.IsNotExist(err) {
		t.Fatalf("expected file %q to be removed, stat error: %v", diskPath, err)
	}
}

func TestServiceDeleteAlbumCoverPropagatesNotFound(t *testing.T) {
	store := newFakeImportStore()
	svc := NewService(store, newRecordingCopier())

	if err := svc.DeleteAlbumCover(ctx(), 7, 999, false); err != ErrAlbumCoverNotFound {
		t.Fatalf("DeleteAlbumCover error = %v, want ErrAlbumCoverNotFound", err)
	}
}

func TestServiceDeleteArtistRemovesArtworkFilesWhenRequested(t *testing.T) {
	store := newCatalogCapturingStore()
	svc := NewService(store, newRecordingCopier())
	dir := t.TempDir()
	svc.SetImages(enrich.NewImageStore(dir))

	if _, err := svc.UploadArtistArt(ctx(), 1, onePixelPNG()); err != nil {
		t.Fatalf("UploadArtistArt: %v", err)
	}
	photo := store.artistPhotos[1][0]
	diskPath := dir + photo.ThumbURL[len("/artwork"):]

	if err := svc.DeleteArtist(ctx(), 1, true); err != nil {
		t.Fatalf("DeleteArtist: %v", err)
	}
	if _, err := os.Stat(diskPath); !os.IsNotExist(err) {
		t.Fatalf("expected artwork file %q to be removed, stat error: %v", diskPath, err)
	}
}

func TestServiceDeleteArtistKeepsArtworkFilesByDefault(t *testing.T) {
	store := newCatalogCapturingStore()
	svc := NewService(store, newRecordingCopier())
	dir := t.TempDir()
	svc.SetImages(enrich.NewImageStore(dir))

	if _, err := svc.UploadArtistArt(ctx(), 1, onePixelPNG()); err != nil {
		t.Fatalf("UploadArtistArt: %v", err)
	}
	photo := store.artistPhotos[1][0]
	diskPath := dir + photo.ThumbURL[len("/artwork"):]

	if err := svc.DeleteArtist(ctx(), 1, false); err != nil {
		t.Fatalf("DeleteArtist: %v", err)
	}
	if _, err := os.Stat(diskPath); err != nil {
		t.Fatalf("expected artwork file %q to survive a keep-file delete, stat error: %v", diskPath, err)
	}
}

func TestServiceDeleteAlbumRemovesArtworkFilesWhenRequested(t *testing.T) {
	store := newCatalogCapturingStore()
	svc := NewService(store, newRecordingCopier())
	dir := t.TempDir()
	svc.SetImages(enrich.NewImageStore(dir))

	if _, err := svc.UploadAlbumArt(ctx(), 7, onePixelPNG()); err != nil {
		t.Fatalf("UploadAlbumArt: %v", err)
	}
	cover := store.albumCovers[7][0]
	diskPath := dir + cover.ThumbURL[len("/artwork"):]

	if err := svc.DeleteAlbum(ctx(), 7, true); err != nil {
		t.Fatalf("DeleteAlbum: %v", err)
	}
	if _, err := os.Stat(diskPath); !os.IsNotExist(err) {
		t.Fatalf("expected artwork file %q to be removed, stat error: %v", diskPath, err)
	}
}
