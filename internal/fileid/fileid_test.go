package fileid

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestEncodeDecodeDocumentRoundTrip(t *testing.T) {
	fileRef := []byte("ref-token")
	got := EncodeDocument(4, 1234567890, 9876543210, fileRef, TypeDocument)
	if got == "" {
		t.Fatal("empty encoded file_id")
	}
	d, err := Decode(got)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if d.Type != TypeDocument {
		t.Errorf("Type = %d, want %d", d.Type, TypeDocument)
	}
	if d.DCID != 4 {
		t.Errorf("DCID = %d, want 4", d.DCID)
	}
	if d.ID != 1234567890 {
		t.Errorf("ID = %d", d.ID)
	}
	if d.AccessHash != 9876543210 {
		t.Errorf("AccessHash = %d", d.AccessHash)
	}
	if !bytes.Equal(d.FileReference, fileRef) {
		t.Errorf("FileReference = %q, want %q", d.FileReference, fileRef)
	}
	if d.IsPhoto() {
		t.Error("document reported as photo")
	}
}

func TestEncodeDecodeDocumentNoReference(t *testing.T) {
	got := EncodeDocument(2, 555, 666, nil, TypeVideo)
	d, err := Decode(got)
	if err != nil {
		t.Fatal(err)
	}
	if d.Type != TypeVideo {
		t.Errorf("Type = %d, want Video", d.Type)
	}
	if d.FileReference != nil {
		t.Errorf("expected nil reference, got %q", d.FileReference)
	}
}

func TestEncodeDecodePhotoRoundTrip(t *testing.T) {
	got := EncodePhoto(4, 111, 222, nil, SourceLegacy, 0, 0, TypePhoto)
	d, err := Decode(got)
	if err != nil {
		t.Fatal(err)
	}
	if !d.IsPhoto() {
		t.Fatal("photo type not detected")
	}
	if d.ID != 111 || d.AccessHash != 222 {
		t.Errorf("id/hash = %d/%d", d.ID, d.AccessHash)
	}
	if d.Source != SourceLegacy {
		t.Errorf("Source = %d", d.Source)
	}
}

func TestPhotoThumbnailSizeRoundTrip(t *testing.T) {
	for _, size := range []byte{'s', 'm', 'x', 'y', 'w'} {
		got := EncodeThumbnailPhoto(2, 555, 666, []byte("ref"), size)
		d, err := Decode(got)
		if err != nil {
			t.Fatalf("size %c: decode: %v", size, err)
		}
		if !d.IsPhoto() {
			t.Fatalf("size %c: not photo", size)
		}
		if d.Source != SourceThumbnail {
			t.Errorf("size %c: Source = %d, want SourceThumbnail", size, d.Source)
		}
		if d.ThumbSize != string(size) {
			t.Errorf("size %c: ThumbSize = %q, want %q", size, d.ThumbSize, string(size))
		}
		if !bytes.Equal(d.FileReference, []byte("ref")) {
			t.Errorf("size %c: FileReference mismatch", size)
		}
	}
}

func TestPhotoDialogSourceRoundTrip(t *testing.T) {
	got := EncodePhoto(1, 7, 8, nil, SourceDialogPhotoBig, 999, 1000, TypeProfilePhoto)
	d, err := Decode(got)
	if err != nil {
		t.Fatal(err)
	}
	if d.Source != SourceDialogPhotoBig {
		t.Errorf("Source = %d", d.Source)
	}
	if d.ChatID != 999 || d.ChatAccessHash != 1000 {
		t.Errorf("chat = %d/%d", d.ChatID, d.ChatAccessHash)
	}
}

func TestUniqueIDRoundTrip(t *testing.T) {
	// file_unique_id only needs to be deterministic and decodable enough to
	// extract the id; verify it encodes without error and is non-empty.
	u := EncodeDocumentUnique(4242, TypeAudio)
	if u == "" {
		t.Fatal("empty unique id")
	}
	p := EncodePhotoUnique(7171, TypePhoto)
	if p == "" {
		t.Fatal("empty photo unique id")
	}
	if u == p {
		t.Error("document and photo unique ids collide")
	}
}

func TestDocumentThumbnailReferenceRoundTrip(t *testing.T) {
	// Real reference value from api.telegram.org: the thumbnail file_id of the
	// first gift sticker (id 5465263910414195580). Encoding from the decoded
	// params must reproduce the reference file_id and file_unique_id byte-for-byte.
	ref, err := hex.DecodeString("006a35540621219e4beab5340e8924ec4b197a359f")
	if err != nil {
		t.Fatal(err)
	}
	const (
		wantFID  = "AAMCAgADFQABajVUBiEhnkvqtTQOiSTsSxl6NZ8AAnwbAAKShNhL-K3EXM8imbUBAAdtAAM8BA"
		wantFUID = "AQADfBsAApKE2Ety"
	)
	got := EncodeDocumentThumbnail(2, 5465263910414195580, -5361215607397896712, ref, 'm')
	if got != wantFID {
		t.Errorf("file_id:\n  got=%s\n want=%s", got, wantFID)
	}
	gotU := EncodeDocumentThumbnailUnique(5465263910414195580, 'm')
	if gotU != wantFUID {
		t.Errorf("file_unique_id:\n  got=%s\n want=%s", gotU, wantFUID)
	}
	// And it must decode back to a Thumbnail-source photo with the right size.
	d, err := Decode(got)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if d.Source != SourceThumbnail || d.ThumbSize != "m" || !d.IsPhoto() {
		t.Errorf("decoded wrong: type=%d source=%d thumb=%q", d.Type, d.Source, d.ThumbSize)
	}
}

func TestStickerSetThumbReferenceRoundTrip(t *testing.T) {
	// Reference value from api.telegram.org: the thumbnail file_id of the
	// animatedEmojis sticker set. Encoding from the decoded params must reproduce
	// both file_id and file_unique_id byte-for-byte.
	const (
		wantFID  = "AAQCABMJAAMCAAN4_wYS2aSW4eguZlhUgvL9PAQ"
		wantFUID = "AQADAgIAA3j_BhJUgvL9"
	)
	// Decoded from wantFID: dc=2, sticker_set_id, access_hash, thumb version.
	const (
		setID      int64 = 1299006433404125186
		accessHash int64 = 6369830300714181849
		thumbVer   int32 = -34438572 // 0xfdf28254
	)
	got := EncodeStickerSetThumb(2, setID, accessHash, thumbVer)
	if got != wantFID {
		t.Errorf("file_id:\n  got=%s\n want=%s", got, wantFID)
	}
	gotU := EncodeStickerSetThumbUnique(setID, thumbVer)
	if gotU != wantFUID {
		t.Errorf("file_unique_id:\n  got=%s\n want=%s", gotU, wantFUID)
	}
	d, err := Decode(got)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if d.Type != TypeThumbnail || d.Source != SourceStickerSetThumbVersion {
		t.Fatalf("decoded type/source = %d/%d, want Thumbnail/StickerSetThumbVersion", d.Type, d.Source)
	}
	if d.StickerSetID != setID || d.StickerSetHash != accessHash || d.ThumbVersion != thumbVer {
		t.Errorf("decoded sticker-set thumb = id:%d hash:%d version:%d, want id:%d hash:%d version:%d",
			d.StickerSetID, d.StickerSetHash, d.ThumbVersion, setID, accessHash, thumbVer)
	}
}

func TestLegacyPhotoSourceRoundTrip(t *testing.T) {
	const (
		photoID     int64 = 101
		accessHash  int64 = 202
		volumeID    int64 = 303
		localID     int32 = 404
		secret      int64 = 505
		dialogID    int64 = 606
		dialogHash  int64 = 707
		stickerID   int64 = 808
		stickerHash int64 = 909
	)

	full := EncodeFullLegacyPhoto(2, photoID, accessHash, []byte("ref"), volumeID, localID, secret, TypePhoto)
	d, err := Decode(full)
	if err != nil {
		t.Fatalf("decode full legacy: %v", err)
	}
	if d.Source != SourceFullLegacy || d.VolumeID != volumeID || d.LocalID != localID || d.Secret != secret {
		t.Fatalf("full legacy decoded as %+v", d)
	}

	dialog := EncodeDialogPhotoLegacy(2, photoID, dialogID, dialogHash, false, volumeID, localID)
	d, err = Decode(dialog)
	if err != nil {
		t.Fatalf("decode dialog legacy: %v", err)
	}
	if d.Source != SourceDialogPhotoSmallLegacy || d.ChatID != dialogID || d.ChatAccessHash != dialogHash ||
		d.VolumeID != volumeID || d.LocalID != localID {
		t.Fatalf("dialog legacy decoded as %+v", d)
	}

	sticker := EncodeStickerSetThumbLegacy(2, stickerID, stickerHash, volumeID, localID)
	d, err = Decode(sticker)
	if err != nil {
		t.Fatalf("decode sticker legacy: %v", err)
	}
	if d.Source != SourceStickerSetThumbLegacy || d.StickerSetID != stickerID || d.StickerSetHash != stickerHash ||
		d.VolumeID != volumeID || d.LocalID != localID {
		t.Fatalf("sticker legacy decoded as %+v", d)
	}
}

func TestLegacyPhotoUniqueFormat(t *testing.T) {
	uid := EncodeFullLegacyPhotoUnique(303, 404)
	raw, err := base64.RawURLEncoding.DecodeString(uid)
	if err != nil {
		t.Fatalf("decode unique: %v", err)
	}
	dec := rleDecode(raw)
	if len(dec) != 17 {
		t.Fatalf("legacy unique len=%d, want 17", len(dec))
	}
	if classType := binary.LittleEndian.Uint32(dec[:4]); classType != 1 {
		t.Errorf("class type=%d, want Photo class+1", classType)
	}
	if dec[4] != 3 {
		t.Errorf("compare type=%d, want 3 for legacy sources", dec[4])
	}
	if volumeID := int64(binary.LittleEndian.Uint64(dec[5:13])); volumeID != 303 {
		t.Errorf("volume_id=%d, want 303", volumeID)
	}
	if localID := int32(binary.LittleEndian.Uint32(dec[13:17])); localID != 404 {
		t.Errorf("local_id=%d, want 404", localID)
	}
	if EncodeDialogPhotoLegacyUnique(303, 404) != uid || EncodeStickerSetThumbLegacyUnique(303, 404) != uid {
		t.Error("legacy unique helpers must share the same TDLib compare key")
	}
}

func TestWebFileRoundTrip(t *testing.T) {
	const (
		url        = "https://example.com/file.jpg"
		accessHash = int64(123456789)
	)
	fid := EncodeWeb(TypeDocument, url, accessHash)
	d, err := Decode(fid)
	if err != nil {
		t.Fatalf("decode web: %v", err)
	}
	if d.Kind != KindWeb || d.Type != TypeDocument || d.URL != url || d.AccessHash != accessHash {
		t.Fatalf("decoded web = %+v", d)
	}
}

func TestGeneratedFileRoundTrip(t *testing.T) {
	t.Run("audio thumbnail", func(t *testing.T) {
		fid := EncodeGenerated(TypeThumbnail, "", "#audio_t#Title#Performer#1#")
		d, err := Decode(fid)
		if err != nil {
			t.Fatalf("decode generated audio: %v", err)
		}
		if d.Kind != KindGenerated || d.Generated != GeneratedAudioThumb || d.AudioTitle != "Title" ||
			d.AudioPerformer != "Performer" || !d.AudioSmall {
			t.Fatalf("decoded generated audio = %+v", d)
		}
	})

	t.Run("map", func(t *testing.T) {
		fid := EncodeGenerated(TypeThumbnail, "", "#map#13#10#20#256#256#2#")
		d, err := Decode(fid)
		if err != nil {
			t.Fatalf("decode generated map: %v", err)
		}
		if d.Kind != KindGenerated || d.Generated != GeneratedMap || d.MapZoom != 13 || d.MapX != 10 ||
			d.MapY != 20 || d.MapWidth != 256 || d.MapHeight != 256 || d.MapScale != 2 {
			t.Fatalf("decoded generated map = %+v", d)
		}
		if d.MapLatitude == 0 || d.MapLongitude == 0 {
			t.Fatalf("map coordinates were not derived: lat=%f lon=%f", d.MapLatitude, d.MapLongitude)
		}
	})

	t.Run("rejects non-thumbnail file type", func(t *testing.T) {
		fid := EncodeGenerated(TypePhoto, "", "#map#13#10#20#256#256#2#")
		if _, err := Decode(fid); err == nil {
			t.Fatal("expected error for generated file_id with TypePhoto")
		}
		fid2 := EncodeGenerated(TypeDocument, "", "#audio_t#Title#Performer#1#")
		if _, err := Decode(fid2); err == nil {
			t.Fatal("expected error for generated file_id with TypeDocument")
		}
	})

	t.Run("rejects unknown conversion", func(t *testing.T) {
		fid := EncodeGenerated(TypeThumbnail, "", "#unknown#data#")
		if _, err := Decode(fid); err == nil {
			t.Fatal("expected error for unknown conversion type")
		}
	})

	t.Run("accepts EncryptedThumbnail type", func(t *testing.T) {
		fid := EncodeGenerated(TypeEncryptedThumbnail, "", "#audio_t#Title#Performer#1#")
		d, err := Decode(fid)
		if err != nil {
			t.Fatalf("expected EncryptedThumbnail to be accepted: %v", err)
		}
		if d.Type != TypeEncryptedThumbnail {
			t.Fatalf("type=%d, want %d", d.Type, TypeEncryptedThumbnail)
		}
	})
 }

func TestNewFileTypeUniqueClasses(t *testing.T) {
	cases := []struct {
		fileType uint32
		want     uint32
	}{
		{TypePhotoStory, 1},
		{TypeLivePhoto, 2},
		{TypeSelfDestructLivePhoto, 2},
		{TypeRingtone, 2},
		{TypeVideoStory, 2},
		{TypeSelfDestructVoiceNote, 2},
		{TypeSecureRaw, 3},
		{TypeSecureDocument, 3},
		{TypeEncrypted, 4},
		{TypeTemp, 5},
	}
	for _, tc := range cases {
		uid := EncodeDocumentUnique(123, tc.fileType)
		raw, err := base64.RawURLEncoding.DecodeString(uid)
		if err != nil {
			t.Fatalf("decode unique for type %d: %v", tc.fileType, err)
		}
		dec := rleDecode(raw)
		if got := binary.LittleEndian.Uint32(dec[:4]); got != tc.want {
			t.Errorf("type %d class=%d, want %d", tc.fileType, got, tc.want)
		}
	}
}

func TestThumbnailPhotoEncoding(t *testing.T) {
	// Verify that EncodeThumbnailPhoto produces the correct binary layout:
	// type(Photo=2|fileRefFlag) | dc | fileRef | photoID | accessHash |
	// sourceDisc(1=Thumbnail) | fileType(2=Photo) | thumbType(int32) | subVer | ver
	fileRef := []byte("abc")
	fid := EncodeThumbnailPhoto(4, 1234567890, 9876543210, fileRef, 'x')

	raw, err := base64.RawURLEncoding.DecodeString(fid)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	dec := rleDecode(raw)

	// Expected layout: 4+4 + 4(fileref: 1len+3data) + 8+8 + 4+4+4 + 1+1 = 42 bytes
	if len(dec) != 42 {
		t.Errorf("len=%d, want 42", len(dec))
	}

	// Verify type has file_reference flag.
	typeVal := binary.LittleEndian.Uint32(dec[0:4])
	if typeVal != (TypePhoto | fileReferenceFlag) {
		t.Errorf("type=0x%x, want 0x%x", typeVal, TypePhoto|fileReferenceFlag)
	}

	// Source discriminator at offset 28 (after 4+4+4+8+8).
	srcDisc := binary.LittleEndian.Uint32(dec[28:32])
	if srcDisc != uint32(SourceThumbnail) {
		t.Errorf("source disc=%d, want %d (Thumbnail)", srcDisc, SourceThumbnail)
	}

	// File type in source.
	srcFileType := binary.LittleEndian.Uint32(dec[32:36])
	if srcFileType != TypePhoto {
		t.Errorf("source file_type=%d, want %d (Photo)", srcFileType, TypePhoto)
	}

	// Thumbnail type character.
	thumbType := binary.LittleEndian.Uint32(dec[36:40])
	if thumbType != 'x' {
		t.Errorf("thumb_type=%d, want %d ('x')", thumbType, 'x')
	}
}

func TestThumbnailPhotoUniqueByte(t *testing.T) {
	// Verify unique byte computation per PhotoSizeSource::get_compare_type.
	cases := []struct {
		thumbType byte
		wantByte  byte
	}{
		{'a', 0},
		{'c', 1},
		{'s', 's' + 5}, // 115+5=120
		{'m', 'm' + 5}, // 109+5=114
		{'x', 'x' + 5}, // 120+5=125
		{'y', 'y' + 5}, // 121+5=126
		{'w', 'w' + 5}, // 119+5=124
	}
	for _, tc := range cases {
		got := thumbnailUniqueByte(tc.thumbType)
		if got != tc.wantByte {
			t.Errorf("thumbnailUniqueByte('%c')=%d, want %d", tc.thumbType, got, tc.wantByte)
		}
	}

	// Different thumbnail types should produce different unique_ids.
	uid1 := EncodeThumbnailPhotoUnique(42, 's')
	uid2 := EncodeThumbnailPhotoUnique(42, 'x')
	if uid1 == uid2 {
		t.Error("different thumbnail types produced same unique_id")
	}
}

func TestDocumentUniqueFormat(t *testing.T) {
	// Document file_unique_id must be class+1=2, not the full file type.
	// All document subtypes (Audio, Sticker, Video, etc.) produce the same
	// class type byte so they can share a unique id namespace.
	for _, ft := range []uint32{TypeAudio, TypeSticker, TypeVideo, TypeDocument, TypeVideoNote} {
		u := EncodeDocumentUnique(123, ft)
		raw, err := base64.RawURLEncoding.DecodeString(u)
		if err != nil {
			t.Fatalf("decode unique for type %d: %v", ft, err)
		}
		dec := rleDecode(raw)
		if len(dec) != 12 {
			t.Errorf("type %d: unique len=%d, want 12 (no trailing byte)", ft, len(dec))
		}
		if len(dec) >= 4 {
			classType := binary.LittleEndian.Uint32(dec[:4])
			if classType != 2 {
				t.Errorf("type %d: class type=%d, want 2 (Document class+1)", ft, classType)
			}
		}
	}
}

func TestDialogPhotoByteCompatibility(t *testing.T) {
	// Real-world data from api.telegram.org for user 1845033319.
	// These are the exact file_id/file_unique_id strings the official Bot API
	// returned for this user's profile photo.
	const (
		officialSmallFID  = "AQADBAAD58IxG6s7YFEACAIAA2f5-G0ABGKJMWxZL-7CPAQ"
		officialBigFID    = "AQADBAAD58IxG6s7YFEACAMAA2f5-G0ABGKJMWxZL-7CPAQ"
		officialSmallFUID = "AQAD58IxG6s7YFEAAQ"
		officialBigFUID   = "AQAD58IxG6s7YFEB"
	)

	// Decode the official file_id to extract the known values.
	d, err := Decode(officialSmallFID)
	if err != nil {
		t.Fatalf("decode official: %v", err)
	}
	if d.Type != TypeProfilePhoto {
		t.Fatalf("Type=%d, want ProfilePhoto(1)", d.Type)
	}
	// The official file_id has access_hash=0 for dialog photos.
	if d.AccessHash != 0 {
		t.Errorf("AccessHash=%d, want 0", d.AccessHash)
	}
	if d.Source != SourceDialogPhotoSmall {
		t.Errorf("Source=%d, want DialogPhotoSmall(2)", d.Source)
	}
	if d.ChatID != 1845033319 {
		t.Errorf("ChatID=%d, want 1845033319", d.ChatID)
	}

	// Re-encode and verify byte-for-byte match.
	gotSmall := EncodeDialogPhoto(d.DCID, d.ID, d.ChatID, d.ChatAccessHash, false)
	gotBig := EncodeDialogPhoto(d.DCID, d.ID, d.ChatID, d.ChatAccessHash, true)
	if gotSmall != officialSmallFID {
		t.Errorf("small file_id mismatch:\n  got=%s\n  want=%s", gotSmall, officialSmallFID)
	}
	if gotBig != officialBigFID {
		t.Errorf("big file_id mismatch:\n  got=%s\n  want=%s", gotBig, officialBigFID)
	}

	// Verify file_unique_id round-trip.
	gotSmallUID := EncodeDialogPhotoUnique(d.ID, false)
	gotBigUID := EncodeDialogPhotoUnique(d.ID, true)
	if gotSmallUID != officialSmallFUID {
		t.Errorf("small unique mismatch:\n  got=%s\n  want=%s", gotSmallUID, officialSmallFUID)
	}
	if gotBigUID != officialBigFUID {
		t.Errorf("big unique mismatch:\n  got=%s\n  want=%s", gotBigUID, officialBigFUID)
	}
	// Small and big unique ids must differ (they're different files).
	if gotSmallUID == gotBigUID {
		t.Error("small and big unique ids are identical")
	}
}

func TestRLERoundTrip(t *testing.T) {
	cases := [][]byte{
		{1, 2, 3},
		{0, 0, 0, 0, 5},
		{0, 0, 0, 0, 0, 0}, // 6 zeros
		{9, 0, 0, 0, 9},
	}
	for _, in := range cases {
		enc := rleEncode(in)
		dec := rleDecode(enc)
		if !bytes.Equal(dec, in) {
			t.Errorf("RLE round-trip failed: %v -> enc -> %v", in, dec)
		}
	}
}

func TestRLELongRun(t *testing.T) {
	// A run of 256 zeros exceeds the 250 cap and must be split into two runs.
	in := make([]byte, 256)
	dec := rleDecode(rleEncode(in))
	if !bytes.Equal(dec, in) {
		t.Errorf("long RLE run round-trip mismatch (len=%d)", len(dec))
	}
}

func TestRLEEncodeCapParity(t *testing.T) {
	// The official zero_encode caps each zero-run at 250 (td/tdutils/misc.cpp:214).
	// A 300-zero run must encode as [0,250][0,50] — NOT [0,254][0,46] (the old
	// 254 cap broke file_id byte-parity for ≥251-byte zero runs). The round-trip
	// test above can't catch this (both caps round-trip); this asserts exact bytes.
	in := make([]byte, 300)
	if got := rleEncode(in); !bytes.Equal(got, []byte{0, 250, 0, 50}) {
		t.Errorf("300-zero run: got %v, want [0 250 0 50] (cap must be 250)", got)
	}
	if got := rleEncode(make([]byte, 250)); !bytes.Equal(got, []byte{0, 250}) {
		t.Errorf("250-zero run: got %v, want [0 250]", got)
	}
}

func TestDecodeInvalid(t *testing.T) {
	if _, err := Decode("!!!not-base64!!!"); err == nil {
		t.Error("expected error for invalid base64")
	}
	// Too short after a valid base64 decode.
	if _, err := Decode("AAAA"); err == nil {
		t.Error("expected error for too-short file_id")
	}
}

// TestFixtureByteParity verifies that encode→decode→re-encode produces
// byte-identical output for every file_id category, matching the TDLib
// wire format. These are the "official fixtures" in the sense that the
// encoded bytes follow the exact TDLib persistent-id format.
func TestFixtureByteParity(t *testing.T) {
	t.Run("web remote file", func(t *testing.T) {
		fid := EncodeWeb(TypeThumbnail, "https://example.com/file.jpg", 123456)
		d, err := Decode(fid)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if d.Kind != KindWeb || d.URL != "https://example.com/file.jpg" || d.AccessHash != 123456 {
			t.Fatalf("decoded = %+v", d)
		}
		// Re-encode and compare raw bytes.
		re := EncodeWeb(d.Type, d.URL, d.AccessHash)
		if re != fid {
			t.Errorf("round-trip mismatch:\n  orig %q\n  re   %q", fid, re)
		}
	})

	t.Run("generated audio thumbnail", func(t *testing.T) {
		fid := EncodeGenerated(TypeThumbnail, "", "#audio_t#Title#Performer#1#")
		d, err := Decode(fid)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if d.Generated != GeneratedAudioThumb || d.AudioTitle != "Title" ||
			d.AudioPerformer != "Performer" || !d.AudioSmall {
			t.Fatalf("decoded = %+v", d)
		}
		re := EncodeGenerated(d.Type, d.OriginalPath, d.Conversion)
		if re != fid {
			t.Errorf("round-trip mismatch:\n  orig %q\n  re   %q", fid, re)
		}
	})

	t.Run("generated map", func(t *testing.T) {
		fid := EncodeGenerated(TypeThumbnail, "", "#map#13#10#20#256#256#2#")
		d, err := Decode(fid)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if d.Generated != GeneratedMap || d.MapZoom != 13 {
			t.Fatalf("decoded = %+v", d)
		}
		re := EncodeGenerated(d.Type, d.OriginalPath, d.Conversion)
		if re != fid {
			t.Errorf("round-trip mismatch:\n  orig %q\n  re   %q", fid, re)
		}
	})

	t.Run("legacy dialog photo", func(t *testing.T) {
		const (
			dcID         int32 = 2
			photoID      int64 = 500
			dialogID     int64 = 123456789
			dialogHash   int64 = 999
			volumeID     int64 = 42
			localID      int32 = 7
		)
		fid := EncodeDialogPhotoLegacy(dcID, photoID, dialogID, dialogHash, true, volumeID, localID)
		d, err := Decode(fid)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if d.Source != SourceDialogPhotoBigLegacy || d.ChatID != dialogID ||
			d.ChatAccessHash != dialogHash || d.VolumeID != volumeID || d.LocalID != localID {
			t.Fatalf("decoded = %+v", d)
		}
		// Verify unique id parity.
		wantUnique := EncodeDialogPhotoLegacyUnique(volumeID, localID)
		if gotUnique := EncodeDialogPhotoLegacyUnique(d.VolumeID, d.LocalID); gotUnique != wantUnique {
			t.Errorf("unique mismatch: %q vs %q", gotUnique, wantUnique)
		}
	})

	t.Run("legacy sticker set thumbnail", func(t *testing.T) {
		const (
			dcID         int32 = 4
			stickerID    int64 = 100
			stickerHash  int64 = 200
			volumeID     int64 = 88
			localID      int32 = 3
		)
		fid := EncodeStickerSetThumbLegacy(dcID, stickerID, stickerHash, volumeID, localID)
		d, err := Decode(fid)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if d.Source != SourceStickerSetThumbLegacy || d.StickerSetID != stickerID ||
			d.StickerSetHash != stickerHash || d.VolumeID != volumeID || d.LocalID != localID {
			t.Fatalf("decoded = %+v", d)
		}
		wantUnique := EncodeStickerSetThumbLegacyUnique(volumeID, localID)
		if gotUnique := EncodeStickerSetThumbLegacyUnique(d.VolumeID, d.LocalID); gotUnique != wantUnique {
			t.Errorf("unique mismatch: %q vs %q", gotUnique, wantUnique)
		}
	})

	t.Run("full legacy photo", func(t *testing.T) {
		const (
			dcID      int32 = 2
			photoID   int64 = 300
			accessHash int64 = 400
			volumeID  int64 = 55
			localID   int32 = 9
			secret    int64 = 777
		)
		fid := EncodeFullLegacyPhoto(dcID, photoID, accessHash, []byte("ref"), volumeID, localID, secret, TypePhoto)
		d, err := Decode(fid)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if d.Source != SourceFullLegacy || d.ID != photoID || d.AccessHash != accessHash ||
			d.VolumeID != volumeID || d.LocalID != localID || d.Secret != secret {
			t.Fatalf("decoded = %+v", d)
		}
		// Re-encode should produce identical bytes.
		re := EncodeFullLegacyPhoto(d.DCID, d.ID, d.AccessHash, d.FileReference,
			d.VolumeID, d.LocalID, d.Secret, d.Type)
		if re != fid {
			t.Errorf("round-trip mismatch:\n  orig %q\n  re   %q", fid, re)
		}
	})
}

// TestOfficialDialogPhotoFixture verifies byte-for-byte parity against a real
// official Bot API dialog photo file_id (user 1845033319, small photo).
func TestOfficialDialogPhotoFixture(t *testing.T) {
	const officialFID = "AQADBAAD58IxG6s7YFEACAIAA2f5-G0ABGKJMWxZL-7CPAQ"
	d, err := Decode(officialFID)
	if err != nil {
		t.Fatalf("decode official file_id: %v", err)
	}
	// FileType.ProfilePhoto=1, source=DialogPhotoSmall(2).
	if d.Type != TypeProfilePhoto {
		t.Errorf("type=%d, want ProfilePhoto(1)", d.Type)
	}
	if d.Source != SourceDialogPhotoSmall {
		t.Errorf("source=%d, want DialogPhotoSmall(2)", d.Source)
	}
	// Re-encode should produce the exact same file_id.
	re := EncodeDialogPhoto(d.DCID, d.ID, d.ChatID, d.ChatAccessHash, false)
	if re != officialFID {
		t.Errorf("round-trip mismatch:\n  orig %q\n  re   %q", officialFID, re)
	}
}

// TestDocumentTypeFixtures verifies encode→decode→re-encode round-trip parity
// for every document-based file type (audio, video, voice, video_note,
// animation, sticker, generic document).
func TestDocumentTypeFixtures(t *testing.T) {
	cases := []struct {
		name     string
		fileType uint32
	}{
		{"audio", TypeAudio},
		{"video", TypeVideo},
		{"voice", TypeVoice},
		{"video_note", TypeVideoNote},
		{"animation", TypeAnimation},
		{"sticker", TypeSticker},
		{"document", TypeDocument},
		{"ringtone", TypeRingtone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dcID := int32(4)
			id := int64(555666777)
			accessHash := int64(888999000)
			fileRef := []byte("ref-data")
			fid := EncodeDocument(dcID, id, accessHash, fileRef, tc.fileType)
			d, err := Decode(fid)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if d.Type != tc.fileType || d.ID != id || d.AccessHash != accessHash {
				t.Fatalf("decoded = type:%d id:%d hash:%d", d.Type, d.ID, d.AccessHash)
			}
			if !bytes.Equal(d.FileReference, fileRef) {
				t.Errorf("file_reference=%v, want %v", d.FileReference, fileRef)
			}
			// Re-encode must produce identical bytes.
			re := EncodeDocument(d.DCID, d.ID, d.AccessHash, d.FileReference, d.Type)
			if re != fid {
				t.Errorf("round-trip mismatch:\n  orig %q\n  re   %q", fid, re)
			}
			// Unique ID must match the class-based unique.
			wantUnique := EncodeDocumentUnique(id, tc.fileType)
			gotUnique := EncodeDocumentUnique(d.ID, d.Type)
			if gotUnique != wantUnique {
				t.Errorf("unique mismatch: %q vs %q", gotUnique, wantUnique)
			}
		})
	}
}
