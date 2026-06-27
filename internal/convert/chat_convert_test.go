package convert

import (
	"testing"

	"github.com/mtgo-labs/mtgo/tg"

	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

func TestDisallowedToAcceptedGifts_Nil(t *testing.T) {
	got := disallowedToAcceptedGifts(nil)
	if !got.UnlimitedGifts || !got.LimitedGifts || !got.UniqueGifts || !got.PremiumSubscription || !got.GiftsFromChannels {
		t.Error("nil disallowed should mean all gifts accepted (all true)")
	}
}

func TestDisallowedToAcceptedGifts_AllDisallowed(t *testing.T) {
	d := &tg.DisallowedGiftsSettings{
		DisallowUnlimitedStargifts:   true,
		DisallowLimitedStargifts:     true,
		DisallowUniqueStargifts:      true,
		DisallowPremiumGifts:         true,
		DisallowStargiftsFromChannels: true,
	}
	got := disallowedToAcceptedGifts(d)
	if got.UnlimitedGifts || got.LimitedGifts || got.UniqueGifts || got.PremiumSubscription || got.GiftsFromChannels {
		t.Error("all-disallowed should mean no gifts accepted (all false)")
	}
}

func TestBusinessIntroToBotAPI(t *testing.T) {
	bi := &tg.BusinessIntro{Title: "Welcome", Description: "My business"}
	out := businessIntroToBotAPI(bi)
	if out == nil || out.Title != "Welcome" || out.Description != "My business" {
		t.Errorf("got %+v", out)
	}
}

func TestBusinessIntroToBotAPI_Nil(t *testing.T) {
	if businessIntroToBotAPI(nil) != nil {
		t.Error("nil should return nil")
	}
}

func TestBusinessLocationToBotAPI(t *testing.T) {
	bl := &tg.BusinessLocation{Address: "123 Main St"}
	out := businessLocationToBotAPI(bl)
	if out == nil || out.Address != "123 Main St" {
		t.Errorf("got %+v", out)
	}
}

func TestBusinessLocationToBotAPI_WithGeo(t *testing.T) {
	bl := &tg.BusinessLocation{
		Address:  "Somewhere",
		GeoPoint: &tg.GeoPoint{Long: 12.34, Lat: 56.78},
	}
	out := businessLocationToBotAPI(bl)
	if out.Location == nil {
		t.Fatal("Location should be set")
	}
	if out.Location.Longitude != 12.34 || out.Location.Latitude != 56.78 {
		t.Errorf("Location = %+v", out.Location)
	}
}

func TestBusinessLocationToBotAPI_Nil(t *testing.T) {
	if businessLocationToBotAPI(nil) != nil {
		t.Error("nil should return nil")
	}
}

func TestBusinessWorkHoursToBotAPI(t *testing.T) {
	bw := &tg.BusinessWorkHours{
		TimezoneID: "UTC",
		WeeklyOpen: []*tg.BusinessWeeklyOpen{{StartMinute: 0, EndMinute: 1440}},
	}
	out := businessWorkHoursToBotAPI(bw)
	if out == nil || out.TimeZoneName != "UTC" {
		t.Errorf("got %+v", out)
	}
	if len(out.OpeningHours) != 1 {
		t.Fatalf("expected 1 interval, got %d", len(out.OpeningHours))
	}
	if out.OpeningHours[0].OpeningMinute != 0 || out.OpeningHours[0].ClosingMinute != 1440 {
		t.Errorf("interval = %+v", out.OpeningHours[0])
	}
}

func TestBusinessWorkHoursToBotAPI_Nil(t *testing.T) {
	if businessWorkHoursToBotAPI(nil) != nil {
		t.Error("nil should return nil")
	}
}

func TestPhotoToChatPhoto(t *testing.T) {
	p := &tg.Photo{ID: 100, DCID: 2}
	out := photoToChatPhoto(p, 123, 456)
	if out == nil {
		t.Fatal("nil output")
	}
	if out.SmallFileID == "" || out.BigFileID == "" {
		t.Error("file IDs should be non-empty")
	}
	if out.SmallFileUniqueID == "" || out.BigFileUniqueID == "" {
		t.Error("file unique IDs should be non-empty")
	}
}

func TestPhotoToChatPhoto_Nil(t *testing.T) {
	if photoToChatPhoto(nil, 1, 2) != nil {
		t.Error("nil photo should return nil")
	}
}

func TestResolveUser_Found(t *testing.T) {
	users := map[int64]*tg.User{
		42: {ID: 42, FirstName: "Alice"},
	}
	out := resolveUser(42, users)
	if out == nil || out.ID != 42 || out.FirstName != "Alice" {
		t.Errorf("got %+v", out)
	}
}

func TestResolveUser_NotFound(t *testing.T) {
	out := resolveUser(999, map[int64]*tg.User{})
	if out == nil || out.ID != 999 {
		t.Errorf("not-found should return User with ID only; got %+v", out)
	}
}

func TestFillAdminRights(t *testing.T) {
	m := &apitypes.ChatMember{}
	r := &tg.ChatAdminRights{
		Other: true, DeleteMessages: true, ChangeInfo: true,
		PinMessages: true, Anonymous: true, ManageTopics: true,
	}
	fillAdminRights(m, r)
	if !m.CanManageChat || !m.CanDeleteMessages || !m.CanChangeInfo {
		t.Error("rights not filled correctly")
	}
	if !m.IsAnonymous || !m.CanManageTopics {
		t.Error("anonymous/topics not filled")
	}
}

func TestFillBannedRights_NotBanned(t *testing.T) {
	m := &apitypes.ChatMember{}
	r := &tg.ChatBannedRights{
		SendMessages: false, SendAudios: false, SendDocs: false,
		SendPhotos: false, SendVideos: false,
	}
	fillBannedRights(m, r)
	// Inverted: false (not banned) → true (allowed).
	if !m.CanSendMessages || !m.CanSendAudios || !m.CanSendDocuments {
		t.Error("not-banned rights should be true")
	}
}

func TestFillBannedRights_Banned(t *testing.T) {
	m := &apitypes.ChatMember{}
	r := &tg.ChatBannedRights{
		SendMessages: true, // banned
	}
	fillBannedRights(m, r)
	if m.CanSendMessages {
		t.Error("banned SendMessages → CanSendMessages should be false")
	}
}

func TestUserProfilePhotos(t *testing.T) {
	photos := &tg.PhotosPhotos{
		Photos: []tg.PhotoClass{
			&tg.Photo{ID: 1, Sizes: []tg.PhotoSizeClass{&tg.PhotoSize{Type: "s", W: 100, H: 100}}},
		},
	}
	out := UserProfilePhotos(photos)
	if out == nil {
		t.Fatal("nil output")
	}
	if out.TotalCount != 1 {
		t.Errorf("TotalCount = %d, want 1", out.TotalCount)
	}
	if len(out.Photos) != 1 {
		t.Fatalf("expected 1 photo, got %d", len(out.Photos))
	}
}

func TestUserProfilePhotos_Empty(t *testing.T) {
	photos := &tg.PhotosPhotos{}
	out := UserProfilePhotos(photos)
	if out == nil {
		t.Fatal("nil output for empty photos")
	}
	if out.TotalCount != 0 || len(out.Photos) != 0 {
		t.Errorf("expected empty, got %+v", out)
	}
}
