package convert

import (
	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/fileid"
	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

// Photo converts a tg.Photo to []types.PhotoSize with proper file_id encoding.
// Each photo size gets a Thumbnail-source file_id keyed by its size type
// character ('s', 'm', 'x', 'y', 'w'), mirroring TDLib's
// FullRemoteFileLocation with PhotoSizeSource::Thumbnail.
func Photo(p *tg.Photo) []apitypes.PhotoSize {
	if p == nil {
		return nil
	}
	sizes := make([]apitypes.PhotoSize, 0, len(p.Sizes))
	for _, s := range p.Sizes {
		sz, ok := s.(*tg.PhotoSize)
		if !ok {
			continue
		}
		thumbType := byte('x') // default to 'x' if Type is empty
		if len(sz.Type) > 0 {
			thumbType = sz.Type[0]
		}
		fid := fileid.EncodeThumbnailPhoto(p.DCID, p.ID, p.AccessHash, p.FileReference, thumbType)
		fuid := fileid.EncodeThumbnailPhotoUnique(p.ID, thumbType)
		sizes = append(sizes, apitypes.PhotoSize{
			FileID:       fid,
			FileUniqueID: fuid,
			Width:        int(sz.W),
			Height:       int(sz.H),
			FileSize:     int(sz.Size),
		})
	}
	return sizes
}

// Document converts a tg.Document to types.Document with proper file_id encoding.
func Document(d *tg.Document) *apitypes.Document {
	if d == nil {
		return nil
	}
	fileType := fileid.DocumentFileType(d)
	fid := fileid.EncodeDocument(d.DCID, d.ID, d.AccessHash, d.FileReference, fileType)
	fuid := fileid.EncodeDocumentUnique(d.ID, fileType)

	doc := &apitypes.Document{
		FileID:       fid,
		FileUniqueID: fuid,
		FileSize:     int(d.Size),
		MimeType:     d.MimeType,
	}
	for _, attr := range d.Attributes {
		if fn, ok := attr.(*tg.DocumentAttributeFilename); ok {
			doc.FileName = fn.FileName
		}
	}
	if th := documentThumbPhotoSize(d); th != nil {
		doc.Thumbnail = th
		doc.Thumb = th
	}
	return doc
}

// Audio converts a tg.Document with audio attributes to types.Audio.
func Audio(d *tg.Document) *apitypes.Audio {
	if d == nil {
		return nil
	}
	fid := fileid.EncodeDocument(d.DCID, d.ID, d.AccessHash, d.FileReference, fileid.TypeAudio)
	fuid := fileid.EncodeDocumentUnique(d.ID, fileid.TypeAudio)

	audio := &apitypes.Audio{
		FileID:       fid,
		FileUniqueID: fuid,
		MimeType:     d.MimeType,
		FileSize:     int(d.Size),
	}
	for _, attr := range d.Attributes {
		switch a := attr.(type) {
		case *tg.DocumentAttributeAudio:
			audio.Duration = int(a.Duration)
			audio.Performer = a.Performer
			audio.Title = a.Title
		case *tg.DocumentAttributeFilename:
			audio.FileName = a.FileName
		}
	}
	if th := documentThumbPhotoSize(d); th != nil {
		audio.Thumbnail = th
		audio.Thumb = th
	}
	return audio
}

// Voice converts a tg.Document with voice attributes to types.Voice.
func Voice(d *tg.Document) *apitypes.Voice {
	if d == nil {
		return nil
	}
	fid := fileid.EncodeDocument(d.DCID, d.ID, d.AccessHash, d.FileReference, fileid.TypeVoice)
	fuid := fileid.EncodeDocumentUnique(d.ID, fileid.TypeVoice)

	v := &apitypes.Voice{
		FileID:       fid,
		FileUniqueID: fuid,
		MimeType:     d.MimeType,
		FileSize:     int(d.Size),
	}
	for _, attr := range d.Attributes {
		if a, ok := attr.(*tg.DocumentAttributeAudio); ok {
			v.Duration = int(a.Duration)
		}
	}
	return v
}

// Video converts a tg.Document with video attributes to types.Video.
func Video(d *tg.Document) *apitypes.Video {
	if d == nil {
		return nil
	}
	fid := fileid.EncodeDocument(d.DCID, d.ID, d.AccessHash, d.FileReference, fileid.TypeVideo)
	fuid := fileid.EncodeDocumentUnique(d.ID, fileid.TypeVideo)

	vid := &apitypes.Video{
		FileID:       fid,
		FileUniqueID: fuid,
		MimeType:     d.MimeType,
		FileSize:     int(d.Size),
	}
	for _, attr := range d.Attributes {
		switch a := attr.(type) {
		case *tg.DocumentAttributeVideo:
			vid.Duration = int(a.Duration)
			vid.Width = int(a.W)
			vid.Height = int(a.H)
		case *tg.DocumentAttributeFilename:
			vid.FileName = a.FileName
		}
	}
	if th := documentThumbPhotoSize(d); th != nil {
		vid.Thumbnail = th
		vid.Thumb = th
	}
	return vid
}

// VideoNote converts a tg.Document with round video attributes to types.VideoNote.
func VideoNote(d *tg.Document) *apitypes.VideoNote {
	if d == nil {
		return nil
	}
	fid := fileid.EncodeDocument(d.DCID, d.ID, d.AccessHash, d.FileReference, fileid.TypeVideoNote)
	fuid := fileid.EncodeDocumentUnique(d.ID, fileid.TypeVideoNote)

	vn := &apitypes.VideoNote{
		FileID:       fid,
		FileUniqueID: fuid,
		FileSize:     int(d.Size),
	}
	for _, attr := range d.Attributes {
		if a, ok := attr.(*tg.DocumentAttributeVideo); ok {
			vn.Duration = int(a.Duration)
			vn.Length = int(a.W) // round videos use W as the diameter
		}
	}
	if th := documentThumbPhotoSize(d); th != nil {
		vn.Thumbnail = th
		vn.Thumb = th
	}
	return vn
}

// Animation converts a tg.Document with animation attributes to types.Animation.
func Animation(d *tg.Document) *apitypes.Animation {
	if d == nil {
		return nil
	}
	fid := fileid.EncodeDocument(d.DCID, d.ID, d.AccessHash, d.FileReference, fileid.TypeAnimation)
	fuid := fileid.EncodeDocumentUnique(d.ID, fileid.TypeAnimation)

	anim := &apitypes.Animation{
		FileID:       fid,
		FileUniqueID: fuid,
		MimeType:     d.MimeType,
		FileSize:     int(d.Size),
	}
	for _, attr := range d.Attributes {
		switch a := attr.(type) {
		case *tg.DocumentAttributeVideo:
			anim.Width = int(a.W)
			anim.Height = int(a.H)
			anim.Duration = int(a.Duration)
		case *tg.DocumentAttributeFilename:
			anim.FileName = a.FileName
		}
	}
	if th := documentThumbPhotoSize(d); th != nil {
		anim.Thumbnail = th
		anim.Thumb = th
	}
	return anim
}

// DocumentMedia dispatches a tg.Document to the appropriate typed converter
// based on its attributes, returning the result as fields on a Message.
func DocumentMedia(d *tg.Document, out *apitypes.Message) {
	if d == nil {
		return
	}
	fileType := fileid.DocumentFileType(d)
	switch fileType {
	case fileid.TypeAudio:
		out.Audio = Audio(d)
	case fileid.TypeVoice:
		out.Voice = Voice(d)
	case fileid.TypeVideo:
		out.Video = Video(d)
	case fileid.TypeVideoNote:
		out.VideoNote = VideoNote(d)
	case fileid.TypeAnimation:
		// The official Bot API returns BOTH animation and document for an animation
		// message (verified via /testall: official keys include animation + document).
		out.Animation = Animation(d)
		out.Document = Document(d)
	case fileid.TypeSticker:
		// StickerFromDocument is the complete converter (type/emoji/dimensions/
		// format/thumbnail); reuse it instead of the minimal Sticker() below.
		out.Sticker = StickerFromDocument(d, nil, "")
	default:
		out.Document = Document(d)
	}
}

// convertPaidMedia builds the Bot API PaidMediaInfo from MTProto extended media
// (Client.cpp JsonPaidMediaInfo:2501 + JsonPaidMedia:2450). Each extended-media
// item maps to one PaidMedia entry: preview / photo / video / other.
func convertPaidMedia(items []tg.MessageExtendedMediaClass, starCount int64) *apitypes.PaidMediaInfo {
	info := &apitypes.PaidMediaInfo{StarCount: int(starCount), PaidMedia: make([]apitypes.PaidMedia, 0, len(items))}
	for _, item := range items {
		switch e := item.(type) {
		case *tg.MessageExtendedMediaPreview:
			info.PaidMedia = append(info.PaidMedia, apitypes.PaidMedia{
				Type:     "preview",
				Width:    int(e.W),
				Height:   int(e.H),
				Duration: int(e.VideoDuration),
			})
		case *tg.MessageExtendedMedia:
			info.PaidMedia = append(info.PaidMedia, paidMediaFromMedia(e.Media))
		}
	}
	return info
}

// paidMediaFromMedia maps a wrapped MessageMediaClass (photo or document) to a
// PaidMedia entry. A photo maps to "photo"; a video document to "video"; anything
// else to "other" (paidMediaUnsupported). The live_photo variant (photo + video)
// is not producible from standard MTProto extended media and is mapped to "photo".
func paidMediaFromMedia(media tg.MessageMediaClass) apitypes.PaidMedia {
	switch m := media.(type) {
	case *tg.MessageMediaPhoto:
		if ph, ok := m.Photo.(*tg.Photo); ok {
			return apitypes.PaidMedia{Type: "photo", Photo: Photo(ph)}
		}
		return apitypes.PaidMedia{Type: "other"}
	case *tg.MessageMediaDocument:
		if d, ok := m.Document.(*tg.Document); ok {
			if fileid.DocumentFileType(d) == fileid.TypeVideo {
				return apitypes.PaidMedia{Type: "video", Video: Video(d)}
			}
		}
		return apitypes.PaidMedia{Type: "other"}
	default:
		return apitypes.PaidMedia{Type: "other"}
	}
}
