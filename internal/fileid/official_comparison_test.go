package fileid

import (
	"bytes"
	"testing"
)

func TestOfficialComparison(t *testing.T) {
	// Real file_ids from official Bot API
	t.Run("profile_photo_thumbnail", func(t *testing.T) {
		// getUserProfilePhotos result — TypePhoto(2), Thumbnail source(1), thumb='a'
		fid := "AgACAgQAAxUAAWo9QmsFVUJR-1pS8AABJCaFFLMAAfkAAuu6MRvGDEFQv38wIPHTbGEBAAMCAANhAAM8BA"
		d, err := Decode(fid)
		if err != nil { t.Fatalf("decode: %v", err) }
		t.Logf("decoded: type=%d dc=%d id=%d access=%d source=%d thumb=%q kind=%d",
			d.Type, d.DCID, d.ID, d.AccessHash, d.Source, d.ThumbSize, d.Kind)
		// Re-encode via EncodeThumbnailPhoto
		re := EncodeThumbnailPhoto(d.DCID, d.ID, d.AccessHash, d.FileReference, d.ThumbSize[0])
		if re != fid {
			t.Errorf("ROUND-TRIP FAIL:\n  orig %q\n  ours %q", fid, re)
		} else {
			t.Log("ROUND-TRIP OK")
		}
	})

	t.Run("sticker_document", func(t *testing.T) {
		fid := "CAACAgIAAxUAAWo9Qvmda7QkwziTaQqHhjtr3QcRAAIEAAN4_wYSZivob5gWHi08BA"
		d, err := Decode(fid)
		if err != nil { t.Fatalf("decode: %v", err) }
		t.Logf("decoded: type=%d dc=%d id=%d access=%d kind=%d",
			d.Type, d.DCID, d.ID, d.AccessHash, d.Kind)
		re := EncodeDocument(d.DCID, d.ID, d.AccessHash, d.FileReference, d.Type)
		if re != fid {
			t.Errorf("ROUND-TRIP FAIL:\n  orig %q\n  ours %q", fid, re)
		} else {
			t.Log("ROUND-TRIP OK")
		}
	})

	t.Run("sticker_set_thumbnail", func(t *testing.T) {
		fid := "AAQCABMJAAMCAAN4_wYSqHF-NKMYHjZUgvL9PAQ"
		d, err := Decode(fid)
		if err != nil { t.Fatalf("decode: %v", err) }
		t.Logf("decoded: type=%d dc=%d id=%d access=%d source=%d stickerSetID=%d stickerSetHash=%d thumbVer=%d kind=%d",
			d.Type, d.DCID, d.ID, d.AccessHash, d.Source, d.StickerSetID, d.StickerSetHash, d.ThumbVersion, d.Kind)
		re := EncodeStickerSetThumb(d.DCID, d.StickerSetID, d.StickerSetHash, d.ThumbVersion)
		if re != fid {
			t.Errorf("ROUND-TRIP FAIL:\n  orig %q\n  ours %q", fid, re)
		} else {
			t.Log("ROUND-TRIP OK")
		}
	})

	// Verify file_unique_id values match official
	t.Run("file_unique_ids", func(t *testing.T) {
		pairs := []struct{ fid, officialUID string }{
			{"AgACAgQAAxUAAWo9QmsFVUJR-1pS8AABJCaFFLMAAfkAAuu6MRvGDEFQv38wIPHTbGEBAAMCAANhAAM8BA", "AQAD67oxG8YMQVAAAQ"},
			{"CAACAgIAAxUAAWo9Qvmda7QkwziTaQqHhjtr3QcRAAIEAAN4_wYSZivob5gWHi08BA", "AgADBAADeP8GEg"},
			{"AAQCABMJAAMCAAN4_wYSqHF-NKMYHjZUgvL9PAQ", "AQADAgIAA3j_BhJUgvL9"},
		}
		for _, p := range pairs {
			d, err := Decode(p.fid)
			if err != nil { t.Errorf("decode %s: %v", p.fid[:20], err); continue }
			var uid string
			switch {
			case d.Source == SourceThumbnail:
				thumb := byte('x')
				if len(d.ThumbSize) > 0 { thumb = d.ThumbSize[0] }
				uid = EncodeThumbnailPhotoUnique(d.ID, thumb)
			case d.Source == SourceStickerSetThumbVersion:
				uid = EncodeStickerSetThumbUnique(d.StickerSetID, d.ThumbVersion)
			default:
				uid = EncodeDocumentUnique(d.ID, d.Type)
			}
			if uid != p.officialUID {
				t.Errorf("unique_id mismatch for %s:\n  official %q\n  ours     %q", p.fid[:20], p.officialUID, uid)
			} else {
				t.Logf("unique_id OK: %q", uid)
			}
			_ = bytes.Equal // keep import
		}
	})
}
