package client

import (
	"errors"
	"testing"

	"github.com/mtgo-labs/mtgo-bot-api/internal/fileid"
	"github.com/mtgo-labs/mtgo/tg"
)

// Regression (D2): a dialog/profile-photo file_id must download via
// inputPeerPhotoFileLocation with the peer reconstructed from the signed dialog
// id (Client.cpp FileLocation.h:415). The file_id stores the Bot API chat_id
// (= MTProto dialog_id), so the peer kind is recovered from the sign.
// Regression (D3): the returned file_unique_id must use EncodeDialogPhotoUnique.
func TestDialogPhotoFileLocationAndUniqueID(t *testing.T) {
	cases := []struct {
		name       string
		dialogID   int64 // = Bot API chat_id
		accessHash int64
		wantPeer   tg.InputPeerClass
	}{
		{"user", 123, 456, &tg.InputPeerUser{UserID: 123, AccessHash: 456}},
		{"basic_group", -77, 0, &tg.InputPeerChat{ChatID: 77}},
		{"channel", -1001234567890, 999, &tg.InputPeerChannel{ChannelID: 1234567890, AccessHash: 999}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fid := fileid.EncodeDialogPhoto(2, 555, tc.dialogID, tc.accessHash, true)
			d, err := fileid.Decode(fid)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			location, err := buildFileLocation(d)
			if err != nil {
				t.Fatalf("buildFileLocation: %v", err)
			}
			loc, ok := location.(*tg.InputPeerPhotoFileLocation)
			if !ok {
				t.Fatalf("want *InputPeerPhotoFileLocation, got %T", location)
			}
			if loc.PhotoID != 555 || !loc.Big {
				t.Errorf("loc = %+v, want PhotoID=555 Big=true", loc)
			}
			if !peerEqual(loc.Peer, tc.wantPeer) {
				t.Errorf("peer = %#v, want %#v", loc.Peer, tc.wantPeer)
			}
			if uid := fileUniqueIDFromDecoded(d); uid != fileid.EncodeDialogPhotoUnique(555, true) {
				t.Errorf("unique id = %q, want EncodeDialogPhotoUnique", uid)
			}
		})
	}
}

func peerEqual(a, b tg.InputPeerClass) bool {
	switch x := a.(type) {
	case *tg.InputPeerUser:
		y, ok := b.(*tg.InputPeerUser)
		return ok && *x == *y
	case *tg.InputPeerChat:
		y, ok := b.(*tg.InputPeerChat)
		return ok && *x == *y
	case *tg.InputPeerChannel:
		y, ok := b.(*tg.InputPeerChannel)
		return ok && *x == *y
	}
	return false
}

// Regression (F8): getFile's file_unique_id for a photo size (and a document
// thumbnail) must match the value the encode path emits for the same file —
// Photo class + photo_id + the PhotoSizeSource::get_compare_type byte — NOT the
// trailer-less document unique id. A file_unique_id is intrinsic to the file, so
// decode(file_id) → unique must round-trip against convert's encode.
func TestPhotoThumbnailUniqueIDRoundTrip(t *testing.T) {
	cases := []struct {
		name      string
		encodeFID func() string
		wantUID   string
	}{
		{
			"photo_size_x",
			func() string { return fileid.EncodeThumbnailPhoto(2, 12345, 999, []byte("ref"), 'x') },
			fileid.EncodeThumbnailPhotoUnique(12345, 'x'),
		},
		{
			"photo_size_y",
			func() string { return fileid.EncodeThumbnailPhoto(4, 777, 1, nil, 'y') },
			fileid.EncodeThumbnailPhotoUnique(777, 'y'),
		},
		{
			"document_thumbnail",
			func() string { return fileid.EncodeDocumentThumbnail(2, 4242, 88, []byte("r"), 'm') },
			fileid.EncodeDocumentThumbnailUnique(4242, 'm'),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := fileid.Decode(tc.encodeFID())
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if uid := fileUniqueIDFromDecoded(d); uid != tc.wantUID {
				t.Errorf("fileUniqueIDFromDecoded = %q, want %q", uid, tc.wantUID)
			}
		})
	}
}

// Regression: sticker-set thumbnails are PhotoSizeSource::StickerSetThumbnailVersion
// in TDLib. They have no photo/document id, download via inputStickerSetThumb,
// and their unique id is built from sticker_set_id + thumb_version.
func TestStickerSetThumbFileLocationAndUniqueID(t *testing.T) {
	const (
		setID      int64 = 1299006433404125186
		accessHash int64 = 6369830300714181849
		thumbVer   int32 = -34438572
	)
	fid := fileid.EncodeStickerSetThumb(2, setID, accessHash, thumbVer)
	d, err := fileid.Decode(fid)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	location, err := buildFileLocation(d)
	if err != nil {
		t.Fatalf("buildFileLocation: %v", err)
	}
	loc, ok := location.(*tg.InputStickerSetThumb)
	if !ok {
		t.Fatalf("want *InputStickerSetThumb, got %T", location)
	}
	if loc.ThumbVersion != thumbVer {
		t.Errorf("ThumbVersion=%d, want %d", loc.ThumbVersion, thumbVer)
	}
	set, ok := loc.Stickerset.(*tg.InputStickerSetID)
	if !ok {
		t.Fatalf("want *InputStickerSetID, got %T", loc.Stickerset)
	}
	if set.ID != setID || set.AccessHash != accessHash {
		t.Errorf("stickerset=%+v, want id=%d access_hash=%d", set, setID, accessHash)
	}
	if uid := fileUniqueIDFromDecoded(d); uid != fileid.EncodeStickerSetThumbUnique(setID, thumbVer) {
		t.Errorf("unique id = %q, want sticker-set thumbnail unique id", uid)
	}
}

func TestLegacyPhotoSources(t *testing.T) {
	const (
		photoID     int64 = 101
		accessHash  int64 = 202
		volumeID    int64 = 303
		localID     int32 = 404
		secret      int64 = 505
		dialogID    int64 = -1001234567890
		dialogHash  int64 = 606
		stickerID   int64 = 707
		stickerHash int64 = 808
	)

	t.Run("full legacy location and unique id", func(t *testing.T) {
		fid := fileid.EncodeFullLegacyPhoto(4, photoID, accessHash, []byte("ref"), volumeID, localID, secret, fileid.TypePhoto)
		d, err := fileid.Decode(fid)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if d.Source != fileid.SourceFullLegacy || d.VolumeID != volumeID || d.LocalID != localID || d.Secret != secret {
			t.Fatalf("decoded legacy photo = source:%d volume:%d local:%d secret:%d", d.Source, d.VolumeID, d.LocalID, d.Secret)
		}
		location, err := buildFileLocation(d)
		if err != nil {
			t.Fatalf("buildFileLocation: %v", err)
		}
		loc, ok := location.(*tg.InputPhotoLegacyFileLocation)
		if !ok {
			t.Fatalf("want *InputPhotoLegacyFileLocation, got %T", location)
		}
		if loc.ID != photoID || loc.AccessHash != accessHash || loc.VolumeID != volumeID || loc.LocalID != localID || loc.Secret != secret {
			t.Errorf("legacy location=%+v", loc)
		}
		if uid := fileUniqueIDFromDecoded(d); uid != fileid.EncodeFullLegacyPhotoUnique(volumeID, localID) {
			t.Errorf("unique id=%q, want full legacy unique", uid)
		}
	})

	t.Run("dialog legacy decode and unique id", func(t *testing.T) {
		fid := fileid.EncodeDialogPhotoLegacy(2, photoID, dialogID, dialogHash, true, volumeID, localID)
		d, err := fileid.Decode(fid)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if d.Source != fileid.SourceDialogPhotoBigLegacy || d.ChatID != dialogID || d.ChatAccessHash != dialogHash ||
			d.VolumeID != volumeID || d.LocalID != localID {
			t.Fatalf("decoded dialog legacy = %+v", d)
		}
		loc, err := buildFileLocation(d)
		if err != nil {
			t.Fatalf("buildFileLocation: %v", err)
		}
		legacy, ok := loc.(*tg.InputPeerPhotoFileLocationLegacy)
		if !ok {
			t.Fatalf("got %T, want *tg.InputPeerPhotoFileLocationLegacy", loc)
		}
		if !legacy.Big || legacy.VolumeID != volumeID || legacy.LocalID != localID {
			t.Fatalf("legacy location = %+v", legacy)
		}
		if uid := fileUniqueIDFromDecoded(d); uid != fileid.EncodeDialogPhotoLegacyUnique(volumeID, localID) {
			t.Errorf("unique id=%q, want dialog legacy unique", uid)
		}
	})

	t.Run("sticker set legacy decode and unique id", func(t *testing.T) {
		fid := fileid.EncodeStickerSetThumbLegacy(2, stickerID, stickerHash, volumeID, localID)
		d, err := fileid.Decode(fid)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if d.Source != fileid.SourceStickerSetThumbLegacy || d.StickerSetID != stickerID || d.StickerSetHash != stickerHash ||
			d.VolumeID != volumeID || d.LocalID != localID {
			t.Fatalf("decoded sticker legacy = %+v", d)
		}
		loc, err := buildFileLocation(d)
		if err != nil {
			t.Fatalf("buildFileLocation: %v", err)
		}
		legacy, ok := loc.(*tg.InputStickerSetThumbLegacy)
		if !ok {
			t.Fatalf("got %T, want *tg.InputStickerSetThumbLegacy", loc)
		}
		if legacy.VolumeID != volumeID || legacy.LocalID != localID {
			t.Fatalf("legacy location = %+v", legacy)
		}
		if uid := fileUniqueIDFromDecoded(d); uid != fileid.EncodeStickerSetThumbLegacyUnique(volumeID, localID) {
			t.Errorf("unique id=%q, want sticker-set legacy unique", uid)
		}
	})
}

func TestWebAndGeneratedFileLocations(t *testing.T) {
	t.Run("web remote", func(t *testing.T) {
		d, err := fileid.Decode(fileid.EncodeWeb(fileid.TypeDocument, "https://example.com/file.bin", 123))
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		location, err := buildFileLocation(d)
		if err != nil {
			t.Fatalf("buildFileLocation: %v", err)
		}
		loc, ok := location.(*tg.InputWebFileLocation)
		if !ok {
			t.Fatalf("want *InputWebFileLocation, got %T", location)
		}
		if loc.URL != "https://example.com/file.bin" || loc.AccessHash != 123 {
			t.Errorf("web location=%+v", loc)
		}
	})

	t.Run("generated audio thumbnail", func(t *testing.T) {
		d, err := fileid.Decode(fileid.EncodeGenerated(fileid.TypeThumbnail, "", "#audio_t#Title#Performer#1#"))
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		location, err := buildFileLocation(d)
		if err != nil {
			t.Fatalf("buildFileLocation: %v", err)
		}
		loc, ok := location.(*tg.InputWebFileAudioAlbumThumbLocation)
		if !ok {
			t.Fatalf("want *InputWebFileAudioAlbumThumbLocation, got %T", location)
		}
		if loc.Title != "Title" || loc.Performer != "Performer" || !loc.Small {
			t.Errorf("audio thumb location=%+v", loc)
		}
	})

	t.Run("generated map builds GeoPointLocation with access_hash=0", func(t *testing.T) {
		d, err := fileid.Decode(fileid.EncodeGenerated(fileid.TypeThumbnail, "", "#map#13#10#20#256#256#2#"))
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		location, err := buildFileLocation(d)
		if err != nil {
			t.Fatalf("buildFileLocation: %v", err)
		}
		loc, ok := location.(*tg.InputWebFileGeoPointLocation)
		if !ok {
			t.Fatalf("want *InputWebFileGeoPointLocation, got %T", location)
		}
		if loc.AccessHash != 0 {
			t.Errorf("access_hash=%d, want 0 (bot accounts never cache location hashes)", loc.AccessHash)
		}
		if loc.W != 256 || loc.H != 256 || loc.Zoom != 13 || loc.Scale != 2 {
			t.Errorf("map params=%+v", loc)
		}
		gp, ok := loc.GeoPoint.(*tg.InputGeoPoint)
		if !ok {
			t.Fatalf("geo point=%T, want *InputGeoPoint", loc.GeoPoint)
		}
		if gp.Lat == 0 || gp.Long == 0 {
			t.Errorf("geo point lat/long not derived: %+v", gp)
		}
	})
}

// A plain document keeps the trailer-less document unique id (Document class + id).
func TestDocumentUniqueIDDefault(t *testing.T) {
	fid := fileid.EncodeDocument(2, 987654, 321, nil, fileid.TypeDocument)
	d, err := fileid.Decode(fid)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if uid := fileUniqueIDFromDecoded(d); uid != fileid.EncodeDocumentUnique(987654, fileid.TypeDocument) {
		t.Errorf("document unique id = %q, want EncodeDocumentUnique", uid)
	}
}

// fileUniqueIDFromDecoded must return empty string for web remote file_ids
// (TDLib's get_unique_file_id skips web locations, FileManager.cpp:1251).
func TestWebUniqueIDEmpty(t *testing.T) {
	d, err := fileid.Decode(fileid.EncodeWeb(fileid.TypeDocument, "https://example.com/f.bin", 999))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if uid := fileUniqueIDFromDecoded(d); uid != "" {
		t.Errorf("web unique id = %q, want empty string", uid)
	}
}

// fileUniqueIDFromDecoded must return EncodeGeneratedUnique for generated
// file_ids (0xFF + serialize format, FileManager.cpp:1213-1214).
func TestGeneratedUniqueID(t *testing.T) {
	d, err := fileid.Decode(fileid.EncodeGenerated(fileid.TypeThumbnail, "", "#audio_t#T#P#1#"))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := fileid.EncodeGeneratedUnique(d.Type, d.OriginalPath, d.Conversion)
	if uid := fileUniqueIDFromDecoded(d); uid != want {
		t.Errorf("generated unique id = %q, want %q", uid, want)
	}
	if uid := fileUniqueIDFromDecoded(d); uid == "" {
		t.Error("generated unique id should not be empty")
	}

	// Map variant.
	dm, err := fileid.Decode(fileid.EncodeGenerated(fileid.TypeThumbnail, "", "#map#13#10#20#256#256#2#"))
	if err != nil {
		t.Fatalf("decode map: %v", err)
	}
	wantM := fileid.EncodeGeneratedUnique(dm.Type, dm.OriginalPath, dm.Conversion)
	if uid := fileUniqueIDFromDecoded(dm); uid != wantM {
		t.Errorf("map unique id = %q, want %q", uid, wantM)
	}
}

// Regression (F9): an upload.getFile result that is an UploadFileCdnRedirect
// must surface errCDNRedirect (so getFile delegates to the CDN-capable download)
// rather than a generic "unavailable" error. A normal UploadFile yields its
// bytes; an unrecognized result type is unavailable.
func TestClassifyGetFileResult(t *testing.T) {
	// Plain file → bytes.
	chunk, err := classifyGetFileResult(&tg.UploadFile{Bytes: []byte("payload")})
	if err != nil || string(chunk) != "payload" {
		t.Fatalf("UploadFile: chunk=%q err=%v", chunk, err)
	}
	// CDN redirect → sentinel (triggers the delegated CDN download).
	if _, err := classifyGetFileResult(&tg.UploadFileCDNRedirect{
		DCID: 4, FileToken: []byte("tok"),
	}); !errors.Is(err, errCDNRedirect) {
		t.Errorf("CDN redirect: err=%v, want errCDNRedirect", err)
	}
	// Unrecognized → unavailable error, not the CDN sentinel.
	if _, err := classifyGetFileResult(nil); err == nil || errors.Is(err, errCDNRedirect) {
		t.Errorf("nil result: err=%v, want a non-CDN unavailable error", err)
	}
}
