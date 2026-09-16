package moderation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/example/go-service/internal/modules/moderation"
)

type fakeProvider struct {
	text  moderation.ProviderResult
	image moderation.ProviderResult
	err   error
}

func (f fakeProvider) CheckText(context.Context, moderation.TextCheckRequest) (moderation.ProviderResult, error) {
	return f.text, f.err
}

func (f fakeProvider) CheckImage(context.Context, moderation.ImageCheckRequest) (moderation.ProviderResult, error) {
	return f.image, f.err
}

func TestNormalizeTextAppliesNFKCAndRemovesZeroWidthAndControls(t *testing.T) {
	got, err := moderation.NormalizeText("  Ａ\u200blice\u0000  ")
	if err != nil || got != "Alice" {
		t.Fatalf("NormalizeText() = %q, %v", got, err)
	}
}

func TestModerateTextMapsRejectedResultWithoutReturningProviderReason(t *testing.T) {
	service := moderation.NewService(fakeProvider{text: moderation.ProviderResult{
		Status: moderation.StatusRejected, ReasonCode: "87014", ProviderRequestID: "provider-id",
	}}, nil)

	result, err := service.ModerateText(context.Background(), 7, "openid", "wechat_authorization", "违规内容")
	if err != nil || result.Status != moderation.StatusRejected || result.ReasonCode != "87014" || result.ProviderRequestID != "provider-id" {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
}

func TestModerateTextFailsClosedOnProviderError(t *testing.T) {
	service := moderation.NewService(fakeProvider{err: errors.New("provider timeout")}, nil)

	result, err := service.ModerateText(context.Background(), 7, "openid", "profile_patch", "玩家")
	if err == nil || result.Status != moderation.StatusUnavailable {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
}

func TestNonApprovedStatusIsNotPublic(t *testing.T) {
	for _, status := range []moderation.Status{moderation.StatusPending, moderation.StatusRejected, moderation.StatusUnreviewed, moderation.StatusUnavailable} {
		if moderation.IsPubliclyApproved(status) {
			t.Fatalf("status %q was public", status)
		}
	}
}
