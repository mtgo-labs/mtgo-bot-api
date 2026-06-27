// Package fileid encodes and decodes the official Telegram Bot API file_id and
// file_unique_id binary format, so file_ids produced here are byte-compatible
// with the official Bot API server and vice-versa.
//
// Wire format (little-endian, base64url-encoded, RLE-compressed zero runs):
//
//	document: type(u32, |1<<25 if file_reference) | dc(u32) | [fileref] |
//	          id(i64) | access_hash(i64) | subVersion(u8=60) | version(u8=4)
//	photo:    type(u32) | dc(u32) | [fileref] | id(i64) | access_hash(i64) |
//	          source(u32) | [chat_id(i64) chat_access_hash(i64) for dialog photos]
//	          | subVersion | version
//
// file_reference is encoded TL-style: 1 length byte if <254, else 254 + 3 bytes,
// padded to a 4-byte boundary.
package fileid

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/mtgo-labs/mtgo/tg"
)

// Sentinel errors for generated file_id validation, matching TDLib's
// from_persistent_id_generated (FileManager.cpp:4013-4019). Callers wrap
// these with "Bad Request: " to produce the official Bot API error text.
var (
	ErrGeneratedFileType    = errors.New("fileid: generated file_id has unsupported file type")
	ErrUnexpectedConversion = errors.New("fileid: unexpected conversion type")
)

// File type ids matching the Bot API binary encoding.
const (
	TypeThumbnail             uint32 = 0
	TypeProfilePhoto          uint32 = 1
	TypePhoto                 uint32 = 2
	TypeVoice                 uint32 = 3
	TypeVideo                 uint32 = 4
	TypeDocument              uint32 = 5
	TypeEncrypted             uint32 = 6
	TypeTemp                  uint32 = 7
	TypeSticker               uint32 = 8
	TypeAudio                 uint32 = 9
	TypeAnimation             uint32 = 10
	TypeEncryptedThumbnail    uint32 = 11
	TypeWallpaper             uint32 = 12
	TypeVideoNote             uint32 = 13
	TypeSecureRaw             uint32 = 14
	TypeSecureDocument        uint32 = 15
	TypeBackground            uint32 = 16
	TypeDocumentPhoto         uint32 = 17
	TypeRingtone              uint32 = 18
	TypeCallLog               uint32 = 19
	TypePhotoStory            uint32 = 20
	TypeVideoStory            uint32 = 21
	TypeSelfDestructPhoto     uint32 = 22
	TypeSelfDestructVideo     uint32 = 23
	TypeSelfDestructVideoNote uint32 = 24
	TypeSelfDestructVoiceNote uint32 = 25
	TypeLivePhoto             uint32 = 26
	TypeSelfDestructLivePhoto uint32 = 27
)

// Photo source types.
const (
	SourceLegacy                 int32 = 0
	SourceThumbnail              int32 = 1
	SourceDialogPhotoSmall       int32 = 2
	SourceDialogPhotoBig         int32 = 3
	SourceStickerSetThumb        int32 = 4
	SourceFullLegacy             int32 = 5
	SourceDialogPhotoSmallLegacy int32 = 6
	SourceDialogPhotoBigLegacy   int32 = 7
	SourceStickerSetThumbLegacy  int32 = 8
)

// Format versions (must match the official Bot API).
const (
	subVersion                 = 60
	version                    = 4
	generatedPersistentVersion = 3

	fileReferenceFlag uint32 = 1 << 25
	webLocationFlag   uint32 = 1 << 24
)

type Kind int

const (
	KindRemote Kind = iota
	KindWeb
	KindGenerated
)

type GeneratedKind int

const (
	GeneratedUnknown GeneratedKind = iota
	GeneratedMap
	GeneratedAudioThumb
)

// Decoded holds the parsed components of a file_id.
type Decoded struct {
	Kind           Kind
	Type           uint32
	DCID           int32
	ID             int64
	AccessHash     int64
	URL            string // for web remote file locations
	FileReference  []byte
	Source         int32  // photo source type
	ChatID         int64  // for dialog photo sources
	ChatAccessHash int64  // for dialog photo sources
	ThumbSize      string // photo size type ('s','m','x','y','w') for thumbnail sources
	StickerSetID   int64  // for sticker set thumbnail sources
	StickerSetHash int64  // for sticker set thumbnail sources
	ThumbVersion   int32  // for sticker set thumbnail version sources
	VolumeID       int64  // for legacy photo sources
	LocalID        int32  // for legacy photo sources
	Secret         int64  // for FullLegacy sources
	OriginalPath   string // for generated file locations
	Conversion     string // for generated file locations
	Generated      GeneratedKind
	MapZoom        int32
	MapX           int32
	MapY           int32
	MapWidth       int32
	MapHeight      int32
	MapScale       int32
	MapLatitude    float64
	MapLongitude   float64
	AudioTitle     string
	AudioPerformer string
	AudioSmall     bool
}

// IsPhoto reports whether the type is photo-like (carries photo source data).
func (d Decoded) IsPhoto() bool {
	switch d.Type {
	case TypePhoto, TypeThumbnail, TypeProfilePhoto:
		return true
	}
	return false
}

// EncodeDocument encodes a document-based file_id (video/audio/voice/sticker/
// animation/video note/generic document).
func EncodeDocument(dcID int32, id, accessHash int64, fileRef []byte, fileType uint32) string {
	var b buf
	typeID := fileType
	if len(fileRef) > 0 {
		typeID |= fileReferenceFlag
	}
	b.u32(typeID)
	b.u32(uint32(dcID))
	if len(fileRef) > 0 {
		b.bytes(fileRef)
	}
	b.i64(id)
	b.i64(accessHash)
	b.u8(subVersion)
	b.u8(version)
	return encode(b.raw())
}

// EncodeWeb encodes a TDLib web remote file location. Web file IDs are
// downloaded with upload.getWebFile instead of upload.getFile.
func EncodeWeb(fileType uint32, url string, accessHash int64) string {
	var b buf
	b.u32(fileType | webLocationFlag)
	b.u32(0)
	b.str(url)
	b.i64(accessHash)
	b.u8(subVersion)
	b.u8(version)
	return encode(b.raw())
}

// EncodeGenerated encodes a TDLib remotely generated thumbnail file_id
// (PERSISTENT_ID_VERSION_GENERATED). TDLib only accepts generated Thumbnail or
// EncryptedThumbnail file types with conversions such as #map#...# and
// #audio_t#...#.
func EncodeGenerated(fileType uint32, originalPath, conversion string) string {
	var b buf
	b.u32(fileType)
	b.str(originalPath)
	b.str(conversion)
	data := rleEncode(b.raw())
	data = append(data, generatedPersistentVersion)
	return base64.RawURLEncoding.EncodeToString(data)
}

// EncodeGeneratedUnique encodes the file_unique_id for a generated file location,
// matching TDLib's FileNode::get_unique_id(FullGenerateFileLocation):
// base64url(rle_encode(0xFF + serialize(file_type + original_path + conversion))).
func EncodeGeneratedUnique(fileType uint32, originalPath, conversion string) string {
	var b buf
	b.u8(0xFF)
	b.u32(fileType)
	b.str(originalPath)
	b.str(conversion)
	return encode(b.raw())
}

// EncodePhoto encodes a photo file_id with source metadata.
func EncodePhoto(dcID int32, photoID, accessHash int64, fileRef []byte,
	sourceType int32, chatID, chatAccessHash int64, photoType uint32,
) string {
	var b buf
	typeID := photoType
	if len(fileRef) > 0 {
		typeID |= fileReferenceFlag
	}
	b.u32(typeID)
	b.u32(uint32(dcID))
	if len(fileRef) > 0 {
		b.bytes(fileRef)
	}
	b.i64(photoID)
	b.i64(accessHash)
	b.u32(uint32(sourceType))
	if sourceType == SourceDialogPhotoSmall || sourceType == SourceDialogPhotoBig {
		b.i64(chatID)
		b.i64(chatAccessHash)
	}
	b.u8(subVersion)
	b.u8(version)
	return encode(b.raw())
}

// fileTypeClassUnique maps a Bot API file type to its FileTypeClass+1 value
// used in file_unique_id encoding (mirrors TDLib's get_file_type_class + 1).
// All document types → 2, all photo types → 1, secure → 3, encrypted → 4, temp → 5.
func fileTypeClassUnique(fileType uint32) uint32 {
	switch fileType {
	case TypeThumbnail, TypeProfilePhoto, TypePhoto, TypeEncryptedThumbnail,
		TypeWallpaper, TypePhotoStory, TypeSelfDestructPhoto:
		return 1 // Photo class + 1
	case TypeVoice, TypeVideo, TypeDocument, TypeSticker, TypeAudio,
		TypeAnimation, TypeVideoNote, TypeBackground, TypeDocumentPhoto,
		TypeRingtone, TypeCallLog, TypeVideoStory, TypeSelfDestructVideo,
		TypeSelfDestructVideoNote, TypeSelfDestructVoiceNote, TypeLivePhoto,
		TypeSelfDestructLivePhoto:
		return 2 // Document class + 1
	case TypeSecureRaw, TypeSecureDocument:
		return 3
	case TypeEncrypted:
		return 4
	case TypeTemp:
		return 5
	default:
		return 2
	}
}

// EncodeDialogPhoto encodes a file_id for a chat/profile photo.
// This is the ProfilePhoto type (1) with DialogPhotoSmall/Big source,
// NOT the regular Photo type (2).
//
// Layout (mirrors TDLib FullRemoteFileLocation::store with DialogPhoto source):
//
//	type(u32 = ProfilePhoto = 1, no file_reference flag) | dc(u32) |
//	photo_id(i64) | access_hash(i64 = 0) |
//	source_discriminator(i32 = 2 small / 3 big) | dialog_id(i64) | dialog_access_hash(i64) |
//	subVersion(u8 = 60) | version(u8 = 4)
//
// access_hash is always 0 for dialog photos because downloading uses
// inputPeerPhotoFileLocation which only needs the peer + photo_id.
func EncodeDialogPhoto(dcID int32, photoID int64, chatID, chatAccessHash int64, isBig bool) string {
	var b buf
	b.u32(TypeProfilePhoto)
	b.u32(uint32(dcID))
	b.i64(photoID)
	b.i64(0) // access_hash = 0 for dialog photos
	if isBig {
		b.u32(uint32(SourceDialogPhotoBig)) // variant index 3
	} else {
		b.u32(uint32(SourceDialogPhotoSmall)) // variant index 2
	}
	b.i64(chatID)
	b.i64(chatAccessHash)
	b.u8(subVersion)
	b.u8(version)
	return encode(b.raw())
}

// EncodeDialogPhotoUnique encodes a file_unique_id for a chat/profile photo.
// Layout: class_type(u32 = Photo class + 1 = 1) | photo_id(i64) | unique_byte(u8).
// The unique byte is 0 for small, 1 for big (from PhotoSizeSource::get_compare_type).
func EncodeDialogPhotoUnique(photoID int64, isBig bool) string {
	var b buf
	b.u32(1) // Photo class + 1
	b.i64(photoID)
	if isBig {
		b.u8(1)
	} else {
		b.u8(0)
	}
	return encode(b.raw())
}

// EncodeThumbnailPhoto encodes a file_id for a regular message photo using the
// Thumbnail source variant. This is the normal encoding for photo sizes in
// messages (sendPhoto, incoming photo messages, etc.).
//
// Layout (mirrors TDLib FullRemoteFileLocation::store with Thumbnail source):
//
//	type(u32 = Photo = 2, with FILE_REFERENCE_FLAG if fileRef present) | dc(u32) |
//	[file_reference] | photo_id(i64) | access_hash(i64) |
//	source_discriminator(i32 = 1 = Thumbnail) | file_type(i32 = 2 = Photo) |
//	thumbnail_type(i32 = size char: 's','m','x','y','w') |
//	subVersion(u8 = 60) | version(u8 = 4)
func EncodeThumbnailPhoto(dcID int32, photoID, accessHash int64, fileRef []byte, thumbnailType byte) string {
	var b buf
	typeID := uint32(TypePhoto)
	if len(fileRef) > 0 {
		typeID |= fileReferenceFlag
	}
	b.u32(typeID)
	b.u32(uint32(dcID))
	if len(fileRef) > 0 {
		b.bytes(fileRef)
	}
	b.i64(photoID)
	b.i64(accessHash)
	b.u32(uint32(SourceThumbnail)) // variant index 1
	b.u32(TypePhoto)               // source file_type = Photo
	b.u32(uint32(thumbnailType))   // PhotoSizeType stored as int32
	b.u8(subVersion)
	b.u8(version)
	return encode(b.raw())
}

// EncodeDocumentThumbnail encodes a file_id for a document's thumbnail (e.g. a
// sticker, animation, or video thumbnail). The layout is identical to
// EncodeThumbnailPhoto, but the outer type and the in-source file type are both
// TypeThumbnail (0) instead of TypePhoto. Verified byte-for-byte against the
// reference Bot API's gift-sticker thumbnail file_ids.
//
// Layout:
//
//	type(u32 = Thumbnail = 0, with FILE_REFERENCE_FLAG if fileRef present) |
//	dc(u32) | [file_reference] | id(i64) | access_hash(i64) |
//	source_discriminator(i32 = 1 = Thumbnail) | file_type(i32 = 0 = Thumbnail) |
//	thumbnail_type(i32 = thumb size char) | subVersion(u8 = 60) | version(u8 = 4)
func EncodeDocumentThumbnail(dcID int32, id, accessHash int64, fileRef []byte, thumbnailType byte) string {
	var b buf
	typeID := uint32(TypeThumbnail)
	if len(fileRef) > 0 {
		typeID |= fileReferenceFlag
	}
	b.u32(typeID)
	b.u32(uint32(dcID))
	if len(fileRef) > 0 {
		b.bytes(fileRef)
	}
	b.i64(id)
	b.i64(accessHash)
	b.u32(uint32(SourceThumbnail)) // variant index 1
	b.u32(TypeThumbnail)           // source file_type = Thumbnail
	b.u32(uint32(thumbnailType))
	b.u8(subVersion)
	b.u8(version)
	return encode(b.raw())
}

// EncodeDocumentThumbnailUnique encodes a file_unique_id for a document
// thumbnail. It shares the Thumbnail-source unique layout with photo thumbnails
// (Photo class + 1, id, compare byte derived from the thumbnail type).
func EncodeDocumentThumbnailUnique(id int64, thumbnailType byte) string {
	return EncodeThumbnailPhotoUnique(id, thumbnailType)
}

// SourceStickerSetThumbVersion is the file_id source discriminator for a sticker
// set thumbnail identified by version (TDLib PhotoSizeSource::Type::StickerSet
// ThumbnailVariant = variant 9 in the file_id source enum). Verified against the
// reference Bot API's getStickerSet thumbnail file_ids.
const SourceStickerSetThumbVersion int32 = 9

// EncodeStickerSetThumb encodes a file_id for a sticker set's thumbnail (the
// StickerSet.thumbnail/thumb field). These thumbnails carry no document id;
// they are identified by the sticker set + a thumbnail version.
//
// Layout (mirrors TDLib FullRemoteFileLocation::store with StickerSetThumbnail
// Version source):
//
//	type(u32 = Thumbnail = 0) | dc(u32) | id(i64 = 0) | access_hash(i64 = 0) |
//	source_discriminator(i32 = 9 = StickerSetThumbnailVersion) |
//	sticker_set_id(i64) | sticker_set_access_hash(i64) | version(i32) |
//	subVersion(u8 = 60) | version(u8 = 4)
func EncodeStickerSetThumb(dcID int32, setID, setAccessHash int64, thumbVersion int32) string {
	var b buf
	b.u32(TypeThumbnail)
	b.u32(uint32(dcID))
	b.i64(0) // set thumbnails carry no document id
	b.i64(0)
	b.u32(uint32(SourceStickerSetThumbVersion))
	b.i64(setID)
	b.i64(setAccessHash)
	b.u32(uint32(thumbVersion))
	b.u8(subVersion)
	b.u8(version)
	return encode(b.raw())
}

// EncodeStickerSetThumbUnique encodes a file_unique_id for a sticker set
// thumbnail. TDLib's StickerSetThumbnailVersion unique (PhotoSizeSource.cpp
// get_unique) is the Photo class, then a 0x02 compare-type tag, the sticker set
// id, and the thumbnail version.
func EncodeStickerSetThumbUnique(setID int64, thumbVersion int32) string {
	var b buf
	b.u32(1) // Photo class + 1
	b.u8(0x02)
	b.i64(setID)
	b.u32(uint32(thumbVersion))
	return encode(b.raw())
}

// EncodeFullLegacyPhoto encodes a current-version TDLib FullLegacy photo
// location. This is mostly needed to accept/reproduce old Bot API file_ids.
func EncodeFullLegacyPhoto(dcID int32, photoID, accessHash int64, fileRef []byte, volumeID int64, localID int32, secret int64, photoType uint32) string {
	var b buf
	typeID := photoType
	if len(fileRef) > 0 {
		typeID |= fileReferenceFlag
	}
	b.u32(typeID)
	b.u32(uint32(dcID))
	if len(fileRef) > 0 {
		b.bytes(fileRef)
	}
	b.i64(photoID)
	b.i64(accessHash)
	b.u32(uint32(SourceFullLegacy))
	b.i64(volumeID)
	b.i64(secret)
	b.u32(uint32(localID))
	b.u8(subVersion)
	b.u8(version)
	return encode(b.raw())
}

// EncodeDialogPhotoLegacy encodes a current-version TDLib legacy dialog photo
// source. Downloading this source requires the historical
// inputPeerPhotoFileLocationLegacy constructor, which may not exist in the
// generated mtgo layer.
func EncodeDialogPhotoLegacy(dcID int32, photoID int64, chatID, chatAccessHash int64, isBig bool, volumeID int64, localID int32) string {
	var b buf
	b.u32(TypeProfilePhoto)
	b.u32(uint32(dcID))
	b.i64(photoID)
	b.i64(0)
	if isBig {
		b.u32(uint32(SourceDialogPhotoBigLegacy))
	} else {
		b.u32(uint32(SourceDialogPhotoSmallLegacy))
	}
	b.i64(chatID)
	b.i64(chatAccessHash)
	b.i64(volumeID)
	b.u32(uint32(localID))
	b.u8(subVersion)
	b.u8(version)
	return encode(b.raw())
}

// EncodeStickerSetThumbLegacy encodes a current-version TDLib legacy sticker set
// thumbnail source. Downloading this source requires the historical
// inputStickerSetThumbLegacy constructor, which may not exist in the generated
// mtgo layer.
func EncodeStickerSetThumbLegacy(dcID int32, setID, setAccessHash int64, volumeID int64, localID int32) string {
	var b buf
	b.u32(TypeThumbnail)
	b.u32(uint32(dcID))
	b.i64(0)
	b.i64(0)
	b.u32(uint32(SourceStickerSetThumbLegacy))
	b.i64(setID)
	b.i64(setAccessHash)
	b.i64(volumeID)
	b.u32(uint32(localID))
	b.u8(subVersion)
	b.u8(version)
	return encode(b.raw())
}

func encodeLegacyPhotoUnique(volumeID int64, localID int32) string {
	var b buf
	b.u32(1) // Photo class + 1
	b.u8(0x03)
	b.i64(volumeID)
	b.u32(uint32(localID))
	return encode(b.raw())
}

// EncodeFullLegacyPhotoUnique encodes the unique id shared by all legacy photo
// sources with the same volume/local pair.
func EncodeFullLegacyPhotoUnique(volumeID int64, localID int32) string {
	return encodeLegacyPhotoUnique(volumeID, localID)
}

// EncodeDialogPhotoLegacyUnique encodes the TDLib unique id for legacy dialog
// photos. Small/big distinction is not part of the legacy compare key.
func EncodeDialogPhotoLegacyUnique(volumeID int64, localID int32) string {
	return encodeLegacyPhotoUnique(volumeID, localID)
}

// EncodeStickerSetThumbLegacyUnique encodes the TDLib unique id for legacy
// sticker-set thumbnails.
func EncodeStickerSetThumbLegacyUnique(volumeID int64, localID int32) string {
	return encodeLegacyPhotoUnique(volumeID, localID)
}

// thumbnailUniqueByte computes the file_unique_id trailer byte for a Thumbnail
// source, mirroring PhotoSizeSource::get_compare_type in PhotoSizeSource.cpp:
// 'a' → 0, 'c' → 1, else type + 5.
func thumbnailUniqueByte(t byte) byte {
	switch t {
	case 'a':
		return 0
	case 'c':
		return 1
	default:
		return t + 5
	}
}

// EncodeThumbnailPhotoUnique encodes a file_unique_id for a regular photo size.
// Layout: class_type(u32 = Photo class + 1 = 1) | photo_id(i64) | unique_byte(u8).
func EncodeThumbnailPhotoUnique(photoID int64, thumbnailType byte) string {
	var b buf
	b.u32(1) // Photo class + 1
	b.i64(photoID)
	b.u8(thumbnailUniqueByte(thumbnailType))
	return encode(b.raw())
}

// EncodeDocumentUnique encodes a file_unique_id for documents.
// Layout: class_type(u32 = Document class + 1 = 2) | id(i64).
// No trailing byte — mirrors TDLib CommonRemoteFileLocation::AsKey::store.
func EncodeDocumentUnique(id int64, fileType uint32) string {
	var b buf
	b.u32(fileTypeClassUnique(fileType))
	b.i64(id)
	return encode(b.raw())
}

// EncodePhotoUnique encodes a file_unique_id for photos with a Thumbnail
// source. Layout: class_type(u32 = Photo class + 1 = 1) | photo_id(i64) | unique_byte(u8).
// The unique byte is derived from the thumbnail type character via
// PhotoSizeSource::get_compare_type: 'a'→0, 'c'→1, else char+5.
func EncodePhotoUnique(photoID int64, photoType uint32) string {
	var b buf
	b.u32(fileTypeClassUnique(photoType))
	b.i64(photoID)
	b.u8(0) // default Thumbnail unique byte
	return encode(b.raw())
}

// Decode parses a Bot API file_id string into its components.
func Decode(fileID string) (Decoded, error) {
	raw, err := base64.RawURLEncoding.DecodeString(fileID)
	if err != nil {
		return Decoded{}, fmt.Errorf("fileid: base64 decode: %w", err)
	}
	if len(raw) > 0 && raw[len(raw)-1] == generatedPersistentVersion {
		return decodeGenerated(raw[:len(raw)-1])
	}
	data := rleDecode(raw)
	if len(data) < 6 {
		return Decoded{}, errors.New("fileid: too short")
	}
	r := bytes.NewReader(data)
	var d Decoded

	typeID, err := readU32(r)
	if err != nil {
		return Decoded{}, fmt.Errorf("fileid: read type: %w", err)
	}
	hasRef := typeID&fileReferenceFlag != 0
	isWeb := typeID&webLocationFlag != 0
	d.Type = typeID &^ (fileReferenceFlag | webLocationFlag)

	dc, err := readU32(r)
	if err != nil {
		return Decoded{}, fmt.Errorf("fileid: read dc: %w", err)
	}
	d.DCID = int32(dc)

	if hasRef {
		ref, err := readBytes(r)
		if err != nil {
			return Decoded{}, fmt.Errorf("fileid: read reference: %w", err)
		}
		d.FileReference = ref
	}
	if isWeb {
		d.Kind = KindWeb
		if d.URL, err = readString(r); err != nil {
			return Decoded{}, fmt.Errorf("fileid: read url: %w", err)
		}
		if d.AccessHash, err = readI64(r); err != nil {
			return Decoded{}, fmt.Errorf("fileid: read access_hash: %w", err)
		}
		return d, nil
	}
	if d.ID, err = readI64(r); err != nil {
		return Decoded{}, fmt.Errorf("fileid: read id: %w", err)
	}
	if d.AccessHash, err = readI64(r); err != nil {
		return Decoded{}, fmt.Errorf("fileid: read access_hash: %w", err)
	}

	if d.IsPhoto() && r.Len() > 2 {
		if src, err := readI32(r); err == nil {
			d.Source = src
			switch src {
			case SourceDialogPhotoSmall, SourceDialogPhotoBig:
				_ = binary.Read(r, binary.LittleEndian, &d.ChatID)
				_ = binary.Read(r, binary.LittleEndian, &d.ChatAccessHash)
			case SourceThumbnail:
				// Layout (EncodeThumbnailPhoto): file_type(i32) | thumbnail_type(i32 = size char).
				var fileType, thumbType int32
				_ = binary.Read(r, binary.LittleEndian, &fileType)
				if binary.Read(r, binary.LittleEndian, &thumbType) == nil {
					d.ThumbSize = string(byte(thumbType))
				}
			case SourceStickerSetThumbVersion:
				_ = binary.Read(r, binary.LittleEndian, &d.StickerSetID)
				_ = binary.Read(r, binary.LittleEndian, &d.StickerSetHash)
				_ = binary.Read(r, binary.LittleEndian, &d.ThumbVersion)
			case SourceFullLegacy:
				_ = binary.Read(r, binary.LittleEndian, &d.VolumeID)
				_ = binary.Read(r, binary.LittleEndian, &d.Secret)
				_ = binary.Read(r, binary.LittleEndian, &d.LocalID)
			case SourceDialogPhotoSmallLegacy, SourceDialogPhotoBigLegacy:
				_ = binary.Read(r, binary.LittleEndian, &d.ChatID)
				_ = binary.Read(r, binary.LittleEndian, &d.ChatAccessHash)
				_ = binary.Read(r, binary.LittleEndian, &d.VolumeID)
				_ = binary.Read(r, binary.LittleEndian, &d.LocalID)
			case SourceStickerSetThumbLegacy:
				_ = binary.Read(r, binary.LittleEndian, &d.StickerSetID)
				_ = binary.Read(r, binary.LittleEndian, &d.StickerSetHash)
				_ = binary.Read(r, binary.LittleEndian, &d.VolumeID)
				_ = binary.Read(r, binary.LittleEndian, &d.LocalID)
			}
		}
	}
	return d, nil
}

func decodeGenerated(raw []byte) (Decoded, error) {
	data := rleDecode(raw)
	r := bytes.NewReader(data)
	var d Decoded
	d.Kind = KindGenerated
	t, err := readU32(r)
	if err != nil {
		return Decoded{}, fmt.Errorf("fileid: read generated type: %w", err)
	}
	d.Type = t
	if d.OriginalPath, err = readString(r); err != nil {
		return Decoded{}, fmt.Errorf("fileid: read generated original_path: %w", err)
	}
	if d.Conversion, err = readString(r); err != nil {
		return Decoded{}, fmt.Errorf("fileid: read generated conversion: %w", err)
	}
	parseGeneratedConversion(&d)
	// TDLib from_persistent_id_generated (FileManager.cpp:4013-4019):
	// The Bot API server passes file_type=Temp, so the type-mismatch clause
	// is always satisfied; only the Thumbnail/EncryptedThumbnail restriction
	// and the remotely-generated-conversion check remain.
	if d.Type != TypeThumbnail && d.Type != TypeEncryptedThumbnail {
		return Decoded{}, ErrGeneratedFileType
	}
	if d.Generated == GeneratedUnknown {
		return Decoded{}, ErrUnexpectedConversion
	}
	return d, nil
}

func parseGeneratedConversion(d *Decoded) {
	parts := strings.Split(d.Conversion, "#")
	if len(parts) < 3 || parts[0] != "" || parts[len(parts)-1] != "" {
		return
	}
	switch {
	case len(parts) == 6 && parts[1] == "audio_t":
		if (parts[2] == "" && parts[3] == "") || (parts[4] != "0" && parts[4] != "1") {
			return
		}
		d.Generated = GeneratedAudioThumb
		d.AudioTitle = parts[2]
		d.AudioPerformer = parts[3]
		d.AudioSmall = parts[4] == "1"
	case len(parts) == 9 && parts[1] == "map":
		zoom, err1 := parseInt32(parts[2])
		x, err2 := parseInt32(parts[3])
		y, err3 := parseInt32(parts[4])
		width, err4 := parseInt32(parts[5])
		height, err5 := parseInt32(parts[6])
		scale, err6 := parseInt32(parts[7])
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil || err5 != nil || err6 != nil {
			return
		}
		size := int32(256 * (1 << zoom))
		if zoom < 13 || zoom > 20 || x < 0 || x >= size || y < 0 || y >= size ||
			width < 16 || width > 1024 || height < 16 || height > 1024 || scale < 1 || scale > 3 {
			return
		}
		d.Generated = GeneratedMap
		d.MapZoom = zoom
		d.MapX = x
		d.MapY = y
		d.MapWidth = width
		d.MapHeight = height
		d.MapScale = scale
		const pi = math.Pi
		d.MapLongitude = (float64(x)+0.1)*360.0/float64(size) - 180
		d.MapLatitude = 90 - 360*math.Atan(math.Exp(((float64(y)+0.1)/float64(size)-0.5)*2*pi))/pi
	}
}

func parseInt32(s string) (int32, error) {
	v, err := strconv.ParseInt(s, 10, 32)
	return int32(v), err
}

// ToInputDocument decodes a file_id into a tg.InputDocument for RPC calls.
func ToInputDocument(fileID string) (*tg.InputDocument, error) {
	d, err := Decode(fileID)
	if err != nil {
		return nil, err
	}
	if d.IsPhoto() {
		return nil, errors.New("fileid: not a document")
	}
	return &tg.InputDocument{ID: d.ID, AccessHash: d.AccessHash, FileReference: refOrEmpty(d.FileReference)}, nil
}

// ToInputPhoto decodes a file_id into a tg.InputPhoto for RPC calls.
func ToInputPhoto(fileID string) (*tg.InputPhoto, error) {
	d, err := Decode(fileID)
	if err != nil {
		return nil, err
	}
	if !d.IsPhoto() {
		return nil, errors.New("fileid: not a photo")
	}
	return &tg.InputPhoto{ID: d.ID, AccessHash: d.AccessHash, FileReference: refOrEmpty(d.FileReference)}, nil
}

// DocumentFileType maps a tg.Document's attributes to the Bot API file type.
func DocumentFileType(doc *tg.Document) uint32 {
	// First pass: dominant "specific" types that must override generic video/audio,
	// regardless of attribute ordering. An animation document carries both
	// DocumentAttributeAnimated and DocumentAttributeVideo; a sticker also carries
	// DocumentAttributeVideo. Without this pass, whichever attribute is iterated first
	// would win and misclassify animations as videos (or stickers as videos).
	for _, attr := range doc.Attributes {
		switch attr.(type) {
		case *tg.DocumentAttributeSticker:
			return TypeSticker
		case *tg.DocumentAttributeAnimated:
			return TypeAnimation
		}
	}
	// Second pass: media types.
	for _, attr := range doc.Attributes {
		switch a := attr.(type) {
		case *tg.DocumentAttributeAudio:
			if a.Voice {
				return TypeVoice
			}
			return TypeAudio
		case *tg.DocumentAttributeVideo:
			if a.RoundMessage {
				return TypeVideoNote
			}
			return TypeVideo
		}
	}
	return TypeDocument
}

func refOrEmpty(ref []byte) []byte {
	if ref == nil {
		return []byte{}
	}
	return ref
}

// --- low-level encoding helpers ---

type buf struct{ b bytes.Buffer }

func (b *buf) u8(v byte) { b.b.WriteByte(v) }

func (b *buf) u32(v uint32) {
	var a [4]byte
	binary.LittleEndian.PutUint32(a[:], v)
	b.b.Write(a[:])
}

func (b *buf) i64(v int64) {
	var a [8]byte
	binary.LittleEndian.PutUint64(a[:], uint64(v))
	b.b.Write(a[:])
}

func (b *buf) str(s string) { b.bytes([]byte(s)) }

// bytes writes a TL-style length-prefixed byte slice, padded to 4 bytes.
func (b *buf) bytes(data []byte) {
	switch {
	case len(data) < 254:
		b.b.WriteByte(byte(len(data)))
		b.b.Write(data)
		for b.b.Len()%4 != 0 {
			b.b.WriteByte(0)
		}
	default:
		b.b.WriteByte(254)
		b.b.WriteByte(byte(len(data)))
		b.b.WriteByte(byte(len(data) >> 8))
		b.b.WriteByte(byte(len(data) >> 16))
		b.b.Write(data)
		for b.b.Len()%4 != 0 {
			b.b.WriteByte(0)
		}
	}
}

func (b *buf) raw() []byte { return b.b.Bytes() }

// --- low-level decoding helpers ---

func readU32(r *bytes.Reader) (uint32, error) {
	var a [4]byte
	if _, err := r.Read(a[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(a[:]), nil
}

func readI32(r *bytes.Reader) (int32, error) {
	u, err := readU32(r)
	return int32(u), err
}

func readI64(r *bytes.Reader) (int64, error) {
	var a [8]byte
	if _, err := r.Read(a[:]); err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(a[:])), nil
}

// readBytes reads a TL-style length-prefixed byte slice (mirrors buf.bytes).
func readBytes(r *bytes.Reader) ([]byte, error) {
	first, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	headerLen := 1
	size := int(first)
	if first == 254 {
		var ln [3]byte
		if _, err := r.Read(ln[:]); err != nil {
			return nil, err
		}
		headerLen = 4
		size = int(ln[0]) | int(ln[1])<<8 | int(ln[2])<<16
	}
	data := make([]byte, size)
	if _, err := r.Read(data); err != nil {
		return nil, err
	}
	for (headerLen+size)%4 != 0 {
		if _, err := r.ReadByte(); err != nil {
			return nil, err
		}
		headerLen++
	}
	return data, nil
}

func readString(r *bytes.Reader) (string, error) {
	data, err := readBytes(r)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// --- RLE compression of zero runs (file_id transport encoding) ---

func encode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(rleEncode(data))
}

func rleEncode(data []byte) []byte {
	var out bytes.Buffer
	for i := 0; i < len(data); i++ {
		if data[i] == 0 {
			count := byte(1)
			for i+int(count) < len(data) && data[i+int(count)] == 0 && count < 250 {
				count++
			}
			out.WriteByte(0)
			out.WriteByte(count)
			i += int(count - 1)
			continue
		}
		out.WriteByte(data[i])
	}
	return out.Bytes()
}

func rleDecode(data []byte) []byte {
	var out bytes.Buffer
	for i := 0; i < len(data); i++ {
		if data[i] == 0 {
			if i+1 < len(data) {
				count := data[i+1]
				for j := 0; j < int(count); j++ {
					out.WriteByte(0)
				}
				i++
			}
			continue
		}
		out.WriteByte(data[i])
	}
	return out.Bytes()
}
