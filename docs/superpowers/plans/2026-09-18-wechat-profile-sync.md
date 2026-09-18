# 微信资料同步接口 Implementation Plan

> For agentic workers: REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Add a server-authoritative WeChat profile sync endpoint that verifies the current account, moderates nickname and avatar data, prevents code replay, and keeps unsafe data out of public responses.

**Architecture:** Keep /api/v1/auth/wechat-login responsible for authentication and new-account initialization, but move existing-account profile changes to the authenticated user route. Reuse one user-module profile application path for both initial account creation and the new sync endpoint; authenticate the endpoint with the current JWT plus a fresh WeChat code2Session openid match. Use Redis for one-time code claims and per-user rate limiting, and reuse migration 00013 and the existing moderation event log without adding a database migration.

**Tech Stack:** Go, Gin, MySQL/sqlc, Redis, WeChat jscode2session, WeChat wxa/msg_sec_check, WeChat wxa/img_sec_check, existing WEBP avatar storage, Go image package, httptest.

**Spec:** docs/superpowers/specs/2026-09-17-wechat-profile-sync-design.md

## Global Constraints

- Only modify backend; do not modify frontend or Godot.
- All API responses use the existing { "code": 0, "message": "success", "data": {} } wrapper.
- AppID and AppSecret remain server environment variables and never appear in responses, logs, Redis values, or frontend files.
- Existing normal nickname PATCH remains disabled; it must not become a bypass for moderation.
- Existing moderation fields and user_moderation_events from migration 00013 remain the source of public-safety state.
- Never save or return rejected, pending, or unavailable raw nickname or avatar data.
- Never accept a client-supplied user ID; the authenticated JWT user ID is authoritative.
- Do not clear Redis or modify rewards, progress, leaderboard scores, or match history.

---

### Task 1: Add Redis one-time code and sync rate-limit primitives

**Files:**
- Modify: backend/internal/platform/redis/keys.go
- Modify: backend/internal/platform/redis/client.go
- Test: backend/internal/platform/redis/keys_test.go
- Test: backend/internal/platform/redis/client_test.go

**Interfaces:**
- Produces WeChatProfileCodeKey(codeHash string) string and WeChatProfileSyncRateKey(userID uint64) string.
- Produces ClaimWeChatProfileCode(ctx context.Context, codeHash string, ttl time.Duration) (bool, error) using atomic SETNX with a short TTL.
- Produces AllowWeChatProfileSync(ctx context.Context, userID uint64, limit int64, window time.Duration) (bool, error) using a per-user counter.
- Does not change refresh-token, avatar-upload, or friend-room key behavior.

- [ ] Step 1: Write a failing unit test for stable hashed keys.

~~~go
func TestWeChatProfileKeysDoNotContainRawCode(t *testing.T) {
    raw := "wx-code-with-sensitive-value"
    key := WeChatProfileCodeKey("hashed-code")
    if strings.Contains(key, raw) || !strings.Contains(key, "wechat-profile") {
        t.Fatalf("key = %q, must contain only the scoped hash", key)
    }
}
~~~

- [ ] Step 2: Run the focused key test and verify it fails because the new key function is absent.

Run: D:/bin/go.exe test ./internal/platform/redis -run TestWeChatProfileKeysDoNotContainRawCode -count=1

Expected: FAIL with an undefined WeChatProfileCodeKey reference.

- [ ] Step 3: Write a failing Redis behavior test using the existing Redis test pattern.

~~~go
func TestClaimWeChatProfileCodeIsAtomic(t *testing.T) {
    client := testRedisClient(t)
    first, err := client.ClaimWeChatProfileCode(context.Background(), "hash-1", time.Minute)
    if err != nil || !first { t.Fatalf("first claim = %v, %v", first, err) }
    second, err := client.ClaimWeChatProfileCode(context.Background(), "hash-1", time.Minute)
    if err != nil || second { t.Fatalf("second claim = %v, %v", second, err) }
}
~~~

- [ ] Step 4: Implement the hashed keys, atomic claim, and rate limiting. Store only 1 for the code claim. Reject zero user IDs, empty hashes, non-positive TTLs, limits, and windows before accessing Redis.
- [ ] Step 5: Run D:/bin/go.exe test ./internal/platform/redis -count=1 and confirm existing token and lock tests still pass.
- [ ] Step 6: Commit the isolated primitive.

~~~powershell
git add backend/internal/platform/redis/keys.go backend/internal/platform/redis/client.go backend/internal/platform/redis/keys_test.go backend/internal/platform/redis/client_test.go
git commit -m "feat: add WeChat profile sync replay guard"
~~~

### Task 2: Build the shared user-module WeChat profile application service

**Files:**
- Create: backend/internal/modules/user/wechat_profile_sync.go
- Create: backend/internal/modules/user/wechat_profile_avatar.go
- Modify: backend/internal/modules/user/dto.go
- Modify: backend/internal/modules/user/errors.go
- Modify: backend/internal/modules/user/service.go
- Modify: backend/internal/modules/user/store.go
- Test: backend/internal/modules/user/wechat_profile_sync_test.go
- Test: backend/internal/modules/user/wechat_profile_avatar_test.go

**Interfaces:**
- Consumes the existing Store, moderation.Service, WeChat client behavior, and AvatarStorage.
- Defines WeChatIdentityStore with GetUserByProviderSubject(ctx, provider, subject string) (db.User, error).
- Defines WeChatProfileClient with ExchangeCode(ctx, code string) (wechatplatform.LoginResult, error).
- Defines WeChatProfileGuard with ClaimWeChatProfileCode(ctx, codeHash string, ttl time.Duration) (bool, error) and AllowWeChatProfileSync(ctx, userID uint64, limit int64, window time.Duration) (bool, error).
- Defines WeChatAvatarFetcher with Fetch(ctx context.Context, rawURL string, maxBytes int64) ([]byte, error).
- Defines WeChatProfileSynchronizer with SyncAuthorizedWeChatProfile(ctx context.Context, userID uint64, subject string, input WeChatProfileInput, source string) (WeChatProfileResult, error), allowing auth to reuse the safe path for a newly created account without exchanging the code twice.
- Produces SyncWeChatProfile(ctx context.Context, userID uint64, input WeChatProfileSyncInput) (WeChatProfileResult, error).

- [ ] Step 1: Add request and response types and write a failing test for rejected nickname preservation.

~~~go
func TestSyncAuthorizedWeChatProfileRejectedNicknameKeepsOldValue(t *testing.T) {
    store := &fakeStore{user: db.User{
        ID: 7, Nickname: "原昵称", Avatar: DefaultAvatar, Status: StatusActive,
        NicknameModerationStatus: string(moderation.StatusApproved),
        AvatarModerationStatus: string(moderation.StatusApproved),
    }}
    service := newProfileSyncService(t, store, moderation.StatusRejected, nil)
    result, err := service.SyncAuthorizedWeChatProfile(context.Background(), 7, "openid-7", WeChatProfileInput{Nickname: "违规昵称"}, "wechat_profile_sync")
    if err != nil { t.Fatal(err) }
    if result.NicknameUpdated || result.Profile.Nickname != "原昵称" || result.SyncStatus != string(moderation.StatusRejected) {
        t.Fatalf("result = %+v", result)
    }
}
~~~

- [ ] Step 2: Run D:/bin/go.exe test ./internal/modules/user -run TestSyncAuthorizedWeChatProfileRejectedNicknameKeepsOldValue -count=1 and confirm it fails because the sync method is absent.
- [ ] Step 3: Implement the shared profile application path. Load the authenticated user, normalize submitted nickname, call ModerateText with verified openid and source, and update the nickname only for approved results. For avatars, fetch only an allowed qlogo HTTPS URL, process it to 256x256 WEBP, call ModerateImage, and save only after approval. Preserve old fields for absent, pending, rejected, unavailable, download, decode, storage, and database failures.
- [ ] Step 4: Record moderation events through the existing moderator without persisting raw rejected or pending content. Aggregate status in this order: unavailable, rejected, pending, approved. Set field-updated booleans only when the stored value changes.
- [ ] Step 5: Implement SyncWeChatProfile. Exchange the code, require a non-empty openid, find the wechat identity, require its user ID to equal the authenticated ID, hash and atomically claim the code in Redis, enforce a per-user rate limit, then call the shared application path. Map invalid code to 401, identity mismatch to 403, guard/store failures to 503, replay to conflict, and rate limiting to 429.
- [ ] Step 6: Run D:/bin/go.exe test ./internal/modules/user -run TestSyncAuthorizedWeChatProfile -count=1 and D:/bin/go.exe test ./internal/modules/user -run TestSyncWeChatProfile -count=1. Confirm approved, pending, rejected, unavailable, identity mismatch, replay, and rate-limit behavior.
- [ ] Step 7: Run D:/bin/go.exe test ./internal/modules/user -count=1.
- [ ] Step 8: Commit the shared sync service.

~~~powershell
git add backend/internal/modules/user/wechat_profile_sync.go backend/internal/modules/user/wechat_profile_avatar.go backend/internal/modules/user/dto.go backend/internal/modules/user/errors.go backend/internal/modules/user/service.go backend/internal/modules/user/store.go backend/internal/modules/user/wechat_profile_sync_test.go backend/internal/modules/user/wechat_profile_avatar_test.go
git commit -m "feat: add moderated WeChat profile sync service"
~~~

### Task 3: Implement safe WeChat avatar downloading

**Files:**
- Modify: backend/internal/modules/user/wechat_profile_avatar.go
- Test: backend/internal/modules/user/wechat_profile_avatar_test.go

**Interfaces:**
- Consumes a raw HTTPS URL only after the WeChat host allowlist check.
- Produces decoded avatar bytes for the existing image processor and never returns a public third-party URL.

- [ ] Step 1: Write failing tests for empty responses, oversized bodies, invalid image content, and redirects outside qlogo.cn.

~~~go
func TestWeChatAvatarFetcherRejectsRedirectOutsideQlogo(t *testing.T) {
    target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("x")) }))
    defer target.Close()
    source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r) {
        http.Redirect(w, r, target.URL, http.StatusFound)
    }))
    defer source.Close()
    fetcher := newWeChatAvatarFetcherForTest(t)
    if _, err := fetcher.Fetch(context.Background(), source.URL, 2<<20); err == nil {
        t.Fatal("Fetch() error = nil, want redirect rejection")
    }
}
~~~

- [ ] Step 2: Run D:/bin/go.exe test ./internal/modules/user -run TestWeChatAvatarFetcher -count=1 and confirm it fails before implementation.
- [ ] Step 3: Implement the fetcher with explicit timeout, HTTPS and qlogo.cn hostname checks, redirect target checks, 2xx requirement, bounded maxBytes+1 reading, and empty-body rejection. Let processAvatarImage verify the real image format and dimensions.
- [ ] Step 4: Run D:/bin/go.exe test ./internal/modules/user -run 'TestWeChatAvatarFetcher|TestUploadAvatar' -count=1 and confirm every failure path preserves the old avatar.
- [ ] Step 5: Commit the fetcher.

~~~powershell
git add backend/internal/modules/user/wechat_profile_avatar.go backend/internal/modules/user/wechat_profile_avatar_test.go
git commit -m "feat: validate and download WeChat avatars safely"
~~~

### Task 4: Expose the authenticated sync route and wire dependencies

**Files:**
- Modify: backend/internal/modules/user/handler.go
- Modify: backend/internal/modules/user/routes.go
- Modify: backend/internal/app/bootstrap.go
- Test: backend/internal/modules/user/handler_test.go
- Test: backend/internal/http/router_test.go

**Interfaces:**
- Produces POST /api/v1/users/me/wechat-profile/sync behind the existing RequireUser middleware and access-token revocation checker.
- Consumes Authorization: Bearer access_token and JSON { "code": "...", "nickname": "...", "avatar": "..." }.
- Returns data.user, data.sync_status, data.nickname_updated, and data.avatar_updated through response.Success.

- [ ] Step 1: Write a failing handler test for the exact path, current-user ownership, and wrapper.

~~~go
func TestSyncWeChatProfileHandlerUsesCurrentUserAndWrappedResponse(t *testing.T) {
    service := newProfileSyncServiceForHandlerTest(t)
    handler := NewHandler(service)
    router := gin.New()
    router.POST("/me/wechat-profile/sync", func(c *gin.Context) {
        c.Set("auth.user_id", uint64(7))
        handler.SyncWeChatProfile(c)
    })
    request := httptest.NewRequest(http.MethodPost, "/me/wechat-profile/sync", strings.NewReader("{\"code\":\"code-1\",\"nickname\":\"新昵称\",\"avatar\":\"\"}"))
    request.Header.Set("Content-Type", "application/json")
    recorder := httptest.NewRecorder()
    router.ServeHTTP(recorder, request)
    if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "\"code\":0") || !strings.Contains(recorder.Body.String(), "\"sync_status\"") {
        t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
    }
}
~~~

- [ ] Step 2: Run D:/bin/go.exe test ./internal/modules/user -run TestSyncWeChatProfileHandlerUsesCurrentUserAndWrappedResponse -count=1 and confirm it fails with an undefined handler method.
- [ ] Step 3: Implement JSON binding, current-user extraction, empty-code validation, and the route. Reject a request with neither nickname nor avatar and never accept user_id from JSON.
- [ ] Step 4: After constructing the WeChat client, content moderator, user service, and Redis client, inject the WeChat client, Redis guard, and production qlogo fetcher into userService. Reuse cfg.WeChat.Timeout and cfg.Avatar.MaxBytes; keep AppSecret inside wechat.Client.
- [ ] Step 5: Run D:/bin/go.exe test ./internal/modules/user ./internal/http -count=1.
- [ ] Step 6: Commit the endpoint and wiring.

~~~powershell
git add backend/internal/modules/user/handler.go backend/internal/modules/user/routes.go backend/internal/app/bootstrap.go backend/internal/modules/user/handler_test.go backend/internal/http/router_test.go
git commit -m "feat: expose authenticated WeChat profile sync endpoint"
~~~

### Task 5: Stop /auth/wechat-login from changing existing profiles

**Files:**
- Modify: backend/internal/modules/auth/service.go
- Modify: backend/internal/modules/auth/wechat_test.go
- Modify: backend/internal/app/bootstrap.go

**Interfaces:**
- Existing /api/v1/auth/wechat-login still exchanges code and issues tokens.
- New accounts may initialize profile data only through the shared moderated synchronizer.
- Existing accounts ignore nickname and avatar fields on login; they must use /users/me/wechat-profile/sync.

- [ ] Step 1: Write a failing regression test for existing-account login immutability and a new-account moderated initializer.

~~~go
func TestLoginWithWeChatExistingAccountDoesNotSyncProfile(t *testing.T) {
    users := existingWeChatUserStoreWithApprovedNickname("原昵称")
    service := newAuthServiceWithProfileSynchronizer(t, users)
    if _, err := service.LoginWithWeChat(context.Background(), WeChatLoginInput{Code: "code", Nickname: "新昵称"}, "127.0.0.1"); err != nil { t.Fatal(err) }
    if users.byIdentity.Nickname != "原昵称" || users.updateCalls != 0 { t.Fatalf("profile changed: %+v", users.byIdentity) }
}
~~~

- [ ] Step 2: Run D:/bin/go.exe test ./internal/modules/auth -run 'TestLoginWithWeChatExistingAccountDoesNotSyncProfile|TestLoginWithWeChat' -count=1 and confirm the regression fails before the change.
- [ ] Step 3: Pass a created/allowInitialProfile flag from loginWithExternal. Create external users with safe default nickname and avatar values, then call the injected user.WeChatProfileSynchronizer with the already verified openid only for a newly created account. For existing accounts, do not call any synchronizer and ignore login profile fields.
- [ ] Step 4: Wire userService as the auth profile synchronizer in bootstrap.go. The synchronizer must not exchange the one-time code again.
- [ ] Step 5: Run D:/bin/go.exe test ./internal/modules/auth ./internal/modules/user -count=1.
- [ ] Step 6: Commit the auth restriction.

~~~powershell
git add backend/internal/modules/auth/service.go backend/internal/modules/auth/wechat_test.go backend/internal/app/bootstrap.go
git commit -m "fix: isolate WeChat profile changes from login"
~~~

### Task 6: Complete security regression tests and backend documentation

**Files:**
- Modify: backend/internal/modules/user/wechat_profile_sync_test.go
- Modify: backend/internal/modules/user/handler_test.go
- Modify: backend/internal/modules/auth/wechat_test.go
- Modify: backend/internal/modules/player/public_profile_test.go
- Modify: backend/docs/openapi.yaml
- Modify: backend/docs/backend-completion.md
- Modify: backend/docs/release-acceptance.md

- [ ] Step 1: Add tests for token expiry/invalid token through middleware, openid mismatch, used code, concurrent same-code requests, rate limiting, invalid source host, cross-host redirect, oversized/empty/invalid image, moderation pending/rejected/unavailable, database update failure, old-avatar preservation, and no existing-account mutation through /auth/wechat-login.
- [ ] Step 2: Run D:/bin/go.exe test ./internal/modules/user ./internal/modules/auth ./internal/modules/player ./internal/http -count=1.
- [ ] Step 3: Add the exact endpoint request and safe response fields to openapi.yaml. Document that production needs existing 00013, Redis, GO_SERVICE_WECHAT_APP_ID, GO_SERVICE_WECHAT_APP_SECRET, and HTTPS server configuration without adding real secret values.
- [ ] Step 4: Scan the diff for frontend/Godot changes and secret leakage. Expected: no frontend/Godot files in implementation commits and no secret values or provider tokens in the new code.
- [ ] Step 5: Commit tests and documentation.

~~~powershell
git add backend/internal/modules/user backend/internal/modules/auth backend/internal/modules/player/public_profile_test.go backend/docs/openapi.yaml backend/docs/backend-completion.md backend/docs/release-acceptance.md
git commit -m "test: cover WeChat profile sync security contract"
~~~

### Task 7: Final verification and handoff

**Files:**
- Verify only; no source changes expected.

- [ ] Step 1: Run gofmt on all modified backend Go files.
- [ ] Step 2: From backend, run D:/bin/go.exe test ./... and confirm PASS.
- [ ] Step 3: From backend, run D:/bin/go.exe vet ./..., D:/bin/go.exe build ./cmd/api, and D:/bin/go.exe build ./cmd/worker.
- [ ] Step 4: Run git status --short and git diff master...HEAD --name-only. Confirm implementation changes are under backend and the user's pre-existing frontend changes remain outside implementation commits.
- [ ] Step 5: Report no new migration, existing 00013 prerequisite, server-side credentials, Redis, API/worker restart, and real-device testing with a fresh wx.login code.

