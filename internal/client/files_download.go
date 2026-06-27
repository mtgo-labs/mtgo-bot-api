package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mtgo-labs/mtgo/telegram/params"
	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/fileid"
	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

func init() {
	Register("getfile", (*Client).getFile)
}

// downloadChunkSize is the max bytes per UploadGetFile call (1 MB).
// MTProto allows up to 1 MB per request.
const downloadChunkSize = 1024 * 1024

// maxDownloadFileSize is the Bot API non-local download cap (Client.h:71;
// enforced Client.cpp:9282/16550). In --local mode there is no cap.
const maxDownloadFileSize = 20 << 20

// getFile downloads a file from Telegram by its file_id and returns a
// types.File with the local file_path.
// Reference: Client.cpp process_get_file_query + do_get_file.
//
// Required parameters: file_id.
func (c *Client) getFile(ctx context.Context, q *server.Query) (any, error) {
	fileID := q.Arg("file_id")
	if fileID == "" {
		return nil, NewError(400, "Bad Request: file_id not specified")
	}

	if err := c.ensureConnected(ctx); err != nil {
		return nil, connError(err)
	}

	decoded, err := fileid.Decode(fileID)
	if err != nil {
		return nil, NewError(400, "Bad Request: invalid file_id")
	}

	// Build the file location for the download RPC.
	location, err := buildFileLocation(decoded)
	if err != nil {
		return nil, err
	}

	tempPath, totalBytes, err := c.downloadToTemp(ctx, decoded, location)
	if err != nil {
		if e, ok := err.(*Error); ok {
			return nil, e // size cap / unavailable — already a Bot API error
		}
		return nil, rpcError(err)
	}

	fileUniqueID := fileUniqueIDFromDecoded(decoded)
	fileName := filepath.Base(tempPath)
	// file_path: --local → absolute local path (the bot reads the file directly);
	// non-local → a path relative to TempDir, served via the
	// /file/bot<token>/<file_path> route (Client.cpp:17411 emits a relative path
	// in non-local mode, absolute in local mode).
	filePath := tempPath
	if !c.params.LocalMode {
		filePath = filepath.Join(c.botID, fileName)
	}
	return &apitypes.File{
		FileID:       fileID,
		FileUniqueID: fileUniqueID,
		FileSize:     totalBytes,
		FilePath:     filePath,
	}, nil
}

// errCDNRedirect is returned by downloadByLocation when upload.getFile replies
// with uploadFileCdnRedirect instead of uploadFile: the file lives on a CDN DC
// that raw RPC cannot reach (mtgo's CDN DC connection + AES-IGE decryption are
// unexported), so getFile delegates the byte-fetch to mtgo's DownloadToFile —
// the raw-RPC rule is relaxed solely for this physically-unreachable case.
var errCDNRedirect = errors.New("file is served via a CDN")

// downloadToTemp fetches the file for decoded/location into a temp file under
// the bot's temp dir and returns its path and size. It tries the raw
// upload.getFile path first (which enforces the non-local 20 MB cap and the
// file_reference refresh), and on a CDN redirect falls back to mtgo's
// CDN-capable DownloadToFile.
func (c *Client) downloadToTemp(ctx context.Context, decoded fileid.Decoded, location any) (string, int, error) {
	botTempDir, err := c.botTempDir()
	if err != nil {
		return "", 0, err
	}

	if webLocation, ok := location.(tg.InputWebFileLocationClass); ok {
		chunks, total, err := c.downloadWebByLocation(ctx, webLocation)
		if err != nil {
			return "", 0, err
		}
		return c.writeChunksToFile(botTempDir, decoded.ID, chunks, total)
	}
	fileLocation, ok := location.(tg.InputFileLocationClass)
	if !ok {
		return "", 0, NewError(400, "Bad Request: invalid file_id")
	}
	chunks, total, err := c.downloadByLocation(ctx, fileLocation)
	// file_reference refresh (F7): re-fetch the originating message and retry once.
	if err != nil && isFileReferenceExpired(err) {
		if fresh, ok := c.tryRefreshFileReference(ctx, decoded); ok {
			decoded.FileReference = fresh
			location, locErr := buildFileLocation(decoded)
			if locErr != nil {
				return "", 0, locErr
			}
			fileLocation, ok = location.(tg.InputFileLocationClass)
			if !ok {
				return "", 0, NewError(400, "Bad Request: invalid file_id")
			}
			chunks, total, err = c.downloadByLocation(ctx, fileLocation)
		}
	}
	switch {
	case err == nil:
		return c.writeChunksToFile(botTempDir, decoded.ID, chunks, total)
	case errors.Is(err, errCDNRedirect):
		location, locErr := buildFileLocation(decoded)
		if locErr != nil {
			return "", 0, locErr
		}
		fileLocation, ok = location.(tg.InputFileLocationClass)
		if !ok {
			return "", 0, NewError(400, "Bad Request: invalid file_id")
		}
		return c.downloadViaCDN(ctx, fileLocation, botTempDir, decoded.ID)
	default:
		return "", 0, err
	}
}

// downloadWebByLocation streams a web file through upload.getWebFile, used by
// TDLib WebRemoteFileLocation and remotely generated web thumbnails.
func (c *Client) downloadWebByLocation(ctx context.Context, location tg.InputWebFileLocationClass) ([][]byte, int, error) {
	var offset int32
	var totalBytes int
	var chunks [][]byte
	for {
		result, err := c.rpc.UploadGetWebFile(ctx, &tg.UploadGetWebFileRequest{
			Location: location,
			Offset:   offset,
			Limit:    downloadChunkSize,
		})
		if err != nil {
			return nil, 0, err
		}
		chunk := result.Bytes
		if len(chunk) == 0 {
			break
		}
		chunks = append(chunks, chunk)
		totalBytes += len(chunk)
		offset += int32(len(chunk))
		if !c.params.LocalMode && totalBytes > maxDownloadFileSize {
			return nil, 0, NewError(400, "Bad Request: file is too big")
		}
		if len(chunk) < downloadChunkSize || result.Size > 0 && totalBytes >= int(result.Size) {
			break
		}
	}
	return chunks, totalBytes, nil
}

// botTempDir resolves and creates the per-bot temp directory, returning its
// absolute path.
func (c *Client) botTempDir() (string, error) {
	tempDir := c.params.TempDir
	if tempDir == "" {
		tempDir = os.TempDir()
	}
	// Local Bot API server: return the ABSOLUTE local path so the bot can read
	// the file directly from disk (no /file/ HTTP download needed), and there is
	// no download size limit. filepath.Abs ensures an absolute path even if
	// TempDir is relative.
	absTempDir, absErr := filepath.Abs(tempDir)
	if absErr != nil {
		absTempDir = tempDir
	}
	botTempDir := filepath.Join(absTempDir, c.botID)
	if err := os.MkdirAll(botTempDir, 0o755); err != nil {
		return "", NewError(500, "Internal Server Error: failed to create temp dir: "+err.Error())
	}
	return botTempDir, nil
}

// writeChunksToFile writes the downloaded chunks to a fresh temp file and
// returns its path and total size.
func (c *Client) writeChunksToFile(botTempDir string, id int64, chunks [][]byte, total int) (string, int, error) {
	fileName := fmt.Sprintf("file_%d_%d", id, total)
	tempPath := filepath.Join(botTempDir, fileName)
	out, err := os.Create(tempPath)
	if err != nil {
		return "", 0, NewError(500, "Internal Server Error: failed to create temp file: "+err.Error())
	}
	defer func() { _ = out.Close() }()
	for _, chunk := range chunks {
		if _, err := out.Write(chunk); err != nil {
			_ = os.Remove(tempPath)
			return "", 0, NewError(500, "Internal Server Error: failed to write file: "+err.Error())
		}
	}
	return tempPath, total, nil
}

// downloadViaCDN fetches a CDN-served file via mtgo's DownloadToFile, which
// faithfully reproduces TDLib's CDN path (connect the CDN DC, upload.getCdnFile,
// the reupload handshake, AES-IGE decryption, and hash verification) plus DC
// migration. The non-local 20 MB cap is enforced by cancelling the download once
// it exceeds the limit.
func (c *Client) downloadViaCDN(ctx context.Context, location tg.InputFileLocationClass, botTempDir string, id int64) (string, int, error) {
	tempPath := filepath.Join(botTempDir, fmt.Sprintf("file_%d_cdn", id))

	dctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var capped bool
	opts := &params.Download{
		Progress: func(info params.ProgressInfo) {
			if !c.params.LocalMode && info.DownloadedBytes > maxDownloadFileSize {
				capped = true
				cancel()
			}
		},
	}
	if err := c.conn.DownloadToFile(dctx, location, tempPath, 0, opts); err != nil {
		_ = os.Remove(tempPath)
		if capped {
			return "", 0, NewError(400, "Bad Request: file is too big")
		}
		return "", 0, err
	}
	info, err := os.Stat(tempPath)
	if err != nil {
		_ = os.Remove(tempPath)
		return "", 0, NewError(500, "Internal Server Error: downloaded file vanished: "+err.Error())
	}
	return tempPath, int(info.Size()), nil
}

// downloadByLocation streams a file from Telegram in 1 MB chunks via
// upload.getFile, returning the gathered chunks and their total size. A
// non-local download is capped at maxDownloadFileSize (Client.cpp:9282). On
// failure it returns the raw RPC error (e.g. FILE_REFERENCE_EXPIRED) so the
// caller can refresh the file_reference and retry; the size-cap and
// unavailable-file cases return an already-formed Bot API *Error.
func (c *Client) downloadByLocation(ctx context.Context, location tg.InputFileLocationClass) ([][]byte, int, error) {
	var offset int64
	var totalBytes int
	var chunks [][]byte
	for {
		req := &tg.UploadGetFileRequest{
			Location: location,
			Offset:   offset,
			Limit:    downloadChunkSize,
		}
		req.SetFlags()

		result, err := c.rpc.UploadGetFile(ctx, req)
		if err != nil {
			return nil, 0, err
		}

		chunk, err := classifyGetFileResult(result)
		if err != nil {
			return nil, 0, err
		}
		if len(chunk) == 0 {
			break
		}
		chunks = append(chunks, chunk)
		totalBytes += len(chunk)
		offset += int64(len(chunk))

		// Non-local download cap (Client.cpp:9282): bail before buffering more.
		if !c.params.LocalMode && totalBytes > maxDownloadFileSize {
			return nil, 0, NewError(400, "Bad Request: file is too big")
		}

		if len(chunk) < downloadChunkSize {
			break // last chunk
		}
	}
	return chunks, totalBytes, nil
}

// classifyGetFileResult extracts the chunk bytes from an upload.getFile result:
// a plain UploadFile yields its bytes, an UploadFileCdnRedirect yields
// errCDNRedirect (so the caller delegates to the CDN path), and anything else is
// treated as an unavailable file.
func classifyGetFileResult(result tg.FileClass) ([]byte, error) {
	switch r := result.(type) {
	case *tg.UploadFile:
		return r.Bytes, nil
	case *tg.UploadFileCDNRedirect:
		return nil, errCDNRedirect
	default:
		return nil, NewError(400, "Bad Request: file is unavailable (storage error)")
	}
}

// buildFileLocation constructs the appropriate InputFileLocationClass from
// a decoded file_id.
func buildFileLocation(d fileid.Decoded) (any, error) {
	if d.Kind == fileid.KindWeb {
		return &tg.InputWebFileLocation{URL: d.URL, AccessHash: d.AccessHash}, nil
	}
	if d.Kind == fileid.KindGenerated {
		switch d.Generated {
		case fileid.GeneratedAudioThumb:
			return &tg.InputWebFileAudioAlbumThumbLocation{
				Small:     d.AudioSmall,
				Title:     d.AudioTitle,
				Performer: d.AudioPerformer,
			}, nil
		case fileid.GeneratedMap:
			// Bot API runs as a bot; TDLib's Location::init (Location.cpp:34)
			// skips caching location access hashes for bots, so
			// G()->get_location_access_hash always returns 0. Map tiles are
			// public static images — access_hash=0 is accepted by upload.getWebFile.
			return &tg.InputWebFileGeoPointLocation{
				GeoPoint:   &tg.InputGeoPoint{Lat: d.MapLatitude, Long: d.MapLongitude},
				AccessHash: 0,
				W:          d.MapWidth,
				H:          d.MapHeight,
				Zoom:       d.MapZoom,
				Scale:      d.MapScale,
			}, nil
		default:
			return nil, NewError(400, "Bad Request: invalid generated file_id")
		}
	}
	ref := refOrEmpty(d.FileReference)
	if d.Source == fileid.SourceFullLegacy {
		return &tg.InputPhotoLegacyFileLocation{
			ID:            d.ID,
			AccessHash:    d.AccessHash,
			FileReference: ref,
			VolumeID:      d.VolumeID,
			LocalID:       d.LocalID,
			Secret:        d.Secret,
		}, nil
	}
	// Dialog/profile photos download via inputPeerPhotoFileLocation (no
	// access_hash/file_reference/thumb), NOT inputPhotoFileLocation
	// (td/td/telegram/files/FileLocation.h:415). The file_id stores a signed
	// dialog id (= Bot API chat_id), so the peer kind is recoverable from its sign.
	if d.Source == fileid.SourceDialogPhotoSmall || d.Source == fileid.SourceDialogPhotoBig {
		return &tg.InputPeerPhotoFileLocation{
			Peer:    dialogIDToInputPeer(d.ChatID, d.ChatAccessHash),
			PhotoID: d.ID,
			Big:     d.Source == fileid.SourceDialogPhotoBig,
		}, nil
	}
	if d.Source == fileid.SourceDialogPhotoSmallLegacy || d.Source == fileid.SourceDialogPhotoBigLegacy {
		return &tg.InputPeerPhotoFileLocationLegacy{
			Big:      d.Source == fileid.SourceDialogPhotoBigLegacy,
			Peer:     dialogIDToInputPeer(d.ChatID, d.ChatAccessHash),
			VolumeID: d.VolumeID,
			LocalID:  d.LocalID,
		}, nil
	}
	if d.Source == fileid.SourceStickerSetThumbVersion {
		return &tg.InputStickerSetThumb{
			Stickerset:   &tg.InputStickerSetID{ID: d.StickerSetID, AccessHash: d.StickerSetHash},
			ThumbVersion: d.ThumbVersion,
		}, nil
	}
	if d.Source == fileid.SourceStickerSetThumbLegacy {
		return &tg.InputStickerSetThumbLegacy{
			Stickerset: &tg.InputStickerSetID{ID: d.StickerSetID, AccessHash: d.StickerSetHash},
			VolumeID:   d.VolumeID,
			LocalID:    d.LocalID,
		}, nil
	}
	if d.IsPhoto() {
		// ThumbSize comes from the file_id (the encoded photo size 's'/'m'/'x'/...);
		// fall back to 'x' (largest crop) only for file_ids that didn't encode one.
		size := d.ThumbSize
		if size == "" {
			size = "x"
		}
		return &tg.InputPhotoFileLocation{
			ID:            d.ID,
			AccessHash:    d.AccessHash,
			FileReference: ref,
			ThumbSize:     size,
		}, nil
	}
	return &tg.InputDocumentFileLocation{
		ID:            d.ID,
		AccessHash:    d.AccessHash,
		FileReference: ref,
		ThumbSize:     "",
	}, nil
}

// dialogIDToInputPeer converts a signed dialog id (= Bot API chat_id, = MTProto
// dialog_id) to the matching InputPeer. user→positive id; basic group→-id;
// channel/supergroup→-(1e12+id).
func dialogIDToInputPeer(dialogID, accessHash int64) tg.InputPeerClass {
	switch {
	case dialogID > 0:
		return &tg.InputPeerUser{UserID: dialogID, AccessHash: accessHash}
	case dialogID <= -1000000000000:
		return &tg.InputPeerChannel{ChannelID: -dialogID - 1000000000000, AccessHash: accessHash}
	default: // -1e12 < dialogID < 0 → basic group
		return &tg.InputPeerChat{ChatID: -dialogID}
	}
}

// fileUniqueIDFromDecoded derives the Bot API file_unique_id matching the
// decoded file's source (FileManager.cpp:1248-1258 returns the unique_id from
// the appropriate file location). A file's file_unique_id is intrinsic — getFile
// must return the same value the photo/thumb carried in its message — so this
// mirrors the TDLib encode path:
//   - generated → EncodeGeneratedUnique (0xFF + serialize, FileManager.cpp:1213-1214)
//   - web remote → "" (TDLib skips web locations in get_unique_file_id, line 1251)
//   - dialog photos → EncodeDialogPhotoUnique
//   - StickerSetThumbnailVersion → EncodeStickerSetThumbUnique
//   - legacy sources → EncodeXxxLegacyUnique
//   - Thumbnail source → EncodeThumbnailPhotoUnique
//   - documents → EncodeDocumentUnique
func fileUniqueIDFromDecoded(d fileid.Decoded) string {
	switch {
	case d.Kind == fileid.KindWeb:
		// TDLib's get_unique_file_id (FileManager.cpp:1251) skips web remote
		// locations — the unique_file_id is empty for web files.
		return ""
	case d.Kind == fileid.KindGenerated:
		return fileid.EncodeGeneratedUnique(d.Type, d.OriginalPath, d.Conversion)
	case d.Source == fileid.SourceDialogPhotoSmall:
		return fileid.EncodeDialogPhotoUnique(d.ID, false)
	case d.Source == fileid.SourceDialogPhotoBig:
		return fileid.EncodeDialogPhotoUnique(d.ID, true)
	case d.Source == fileid.SourceStickerSetThumbVersion:
		return fileid.EncodeStickerSetThumbUnique(d.StickerSetID, d.ThumbVersion)
	case d.Source == fileid.SourceFullLegacy:
		return fileid.EncodeFullLegacyPhotoUnique(d.VolumeID, d.LocalID)
	case d.Source == fileid.SourceDialogPhotoSmallLegacy || d.Source == fileid.SourceDialogPhotoBigLegacy:
		return fileid.EncodeDialogPhotoLegacyUnique(d.VolumeID, d.LocalID)
	case d.Source == fileid.SourceStickerSetThumbLegacy:
		return fileid.EncodeStickerSetThumbLegacyUnique(d.VolumeID, d.LocalID)
	case d.Source == fileid.SourceThumbnail:
		thumb := byte('x')
		if len(d.ThumbSize) > 0 {
			thumb = d.ThumbSize[0]
		}
		return fileid.EncodeThumbnailPhotoUnique(d.ID, thumb)
	default:
		return fileid.EncodeDocumentUnique(d.ID, d.Type)
	}
}

// refOrEmpty returns an empty (non-nil) byte slice if ref is nil, matching
// the MTProto expectation that file_reference is always present.
func refOrEmpty(ref []byte) []byte {
	if ref == nil {
		return []byte{}
	}
	return ref
}
