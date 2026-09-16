# 用户资料内容审核实施计划

> For agentic workers: REQUIRED SUB-SKILL: Use superpowers:executing-plans (recommended for inline execution) to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** 在不修改前端和 Godot 的前提下，为“三火算术练习”建立服务端最终裁决的昵称/头像审核、公开资料过滤、审核审计和历史整改能力。

**Architecture:** 微信平台客户端负责服务端 access token 缓存以及 msg_sec_check/img_sec_check HTTP 协议；独立 moderation 模块负责归一化、状态和 fail-closed 决策；auth/user/player 只通过接口消费审核结果。用户资料状态落在 MySQL，公开资料在 SQL 读取和 Redis 房间响应两个边界都过滤，头像审核通过后才进入现有公开存储。

**Tech Stack:** Go 1.26、Gin、MySQL/Goose、现有手写 sqlc 产物、Redis、微信官方内容安全 HTTP API、httptest.Server。

**Spec:** backend/docs/superpowers/specs/2026-09-16-content-moderation-design.md

## Global Constraints

- 只修改 D:/微信小游戏/backend；不修改 frontend、Godot 或前端提示词。
- AppSecret、微信接口 access token、用户 access_token、refresh_token 和完整原始图片不能写入日志、响应或仓库。
- 所有接口继续使用现有 code/message/data 响应包装；历史数字错误码保持兼容。
- 非 approved 的昵称/头像不得出现在排行榜、好友房、匹配、排位历史或公开资料响应。
- 审核失败不覆盖旧资料、不删除账号/进度/金币/对战/排位历史、不产生奖励。
- 审核上游超时、配置错误、数据库错误必须 fail closed；不能静默当作通过。
- 不执行全库 Redis 清空；整改任务只处理审核模块自有缓存或依赖读取时的安全过滤。
- 每个实现任务先写测试、运行测试确认按预期失败，再写最小生产代码，最后重新运行测试。

---

### Task 1: 增加可兼容的业务错误码和 moderation 核心类型

**Files:**
- Create: backend/internal/modules/moderation/types.go
- Create: backend/internal/modules/moderation/normalize.go
- Create: backend/internal/modules/moderation/service.go
- Create: backend/internal/modules/moderation/service_test.go
- Modify: backend/internal/apperror/errors.go
- Modify: backend/internal/http/response/error.go
- Test: backend/internal/http/response/error_test.go

**Interfaces:**
- Produces moderation.Provider, moderation.Decision, moderation.Service.ModerateText, moderation.Service.ModerateImage, moderation.NormalizeText and moderation.IsPubliclyApproved.
- Produces apperror.NewBusiness(code string, status int, message string, err error); existing numeric AppError callers remain valid.

- [ ] Step 1: Write the failing tests.

Use a real in-memory fake provider:

~~~go
type fakeProvider struct {
    text moderation.ProviderResult
    image moderation.ProviderResult
    err error
}

func TestNormalizeTextAppliesNFKCAndRemovesZeroWidthAndControls(t *testing.T) {}
func TestModerateTextMapsRejectedResultWithoutReturningProviderReason(t *testing.T) {}
func TestModerateTextFailsClosedOnProviderError(t *testing.T) {}
func TestNonApprovedStatusIsNotPublic(t *testing.T) {}
~~~

Assert that full-width text becomes normalized ASCII, rejected results preserve only an internal reason code, provider errors return StatusUnavailable, and pending/rejected/unreviewed are never publicly approved. Add a response test asserting NICKNAME_EDIT_DISABLED serializes as a string code while BadRequest still serializes as numeric 10001.

- [ ] Step 2: Run the focused tests and verify the expected failure.

Run from D:/微信小游戏/backend:

~~~powershell
D:/bin/go.exe test ./internal/modules/moderation ./internal/http/response -run 'TestNormalizeText|TestModerateText|TestNonApprovedStatus|TestBusiness' -count=1
~~~

Expected: FAIL because the moderation package, provider result, and string business-code response support do not exist.

- [ ] Step 3: Implement the minimal core.

Define statuses approved, pending, rejected, unreviewed and unavailable; define ProviderResult, Provider, AuditEvent, AuditStore and Decision. Normalize with golang.org/x/text/unicode/norm.NFKC, remove unicode.Cf and control runes, collapse whitespace, and reject empty/overlong text.

Extend AppError with BusinessCode string. NewBusiness sets the string code and HTTP status while retaining a numeric fallback. Change only response.ErrorResponse.Code to any; WriteError emits BusinessCode when present, otherwise the existing numeric code.

ModerateText and ModerateImage call the provider, map provider errors to StatusUnavailable, and never include provider reason text in client-facing errors. When AuditStore is configured, record only user ID, resource type, source, status, reason code and provider request ID.

- [ ] Step 4: Run the focused tests and verify they pass.

~~~powershell
D:/bin/go.exe test ./internal/modules/moderation ./internal/http/response -run 'TestNormalizeText|TestModerateText|TestNonApprovedStatus|TestBusiness' -count=1
~~~

Expected: PASS.

- [ ] Step 5: Format and commit.

~~~powershell
Get-ChildItem internal/modules/moderation -Filter *.go | ForEach-Object { gofmt -w $_.FullName }
gofmt -w internal/apperror/errors.go internal/http/response/error.go internal/http/response/error_test.go
D:/bin/go.exe test ./internal/modules/moderation ./internal/http/response -count=1
git add internal/modules/moderation internal/apperror/errors.go internal/http/response/error.go internal/http/response/error_test.go
git commit -m "feat: add moderation decisions and business error codes"
~~~

### Task 2: Implement the WeChat content-safety client

**Files:**
- Create: backend/internal/platform/wechat/content_safety.go
- Create: backend/internal/platform/wechat/content_safety_test.go
- Modify: backend/internal/platform/wechat/client.go

**Interfaces:**
- Implements moderation.Provider through wechat.Client.
- Uses existing config.WeChatConfig AppID/AppSecret/APIBaseURL/Timeout and never returns either secret or service access token.

- [ ] Step 1: Write failing HTTP contract tests.

Use httptest.Server and assert:

~~~go
func TestContentSafetyClientChecksTextAndCachesServerToken(t *testing.T) {}
func TestContentSafetyClientChecksImageWithMultipartMedia(t *testing.T) {}
func TestContentSafetyClientMaps87014ToRejected(t *testing.T) {}
func TestContentSafetyClientDoesNotExposeSecretInErrorOrLog(t *testing.T) {}
func TestContentSafetyClientRefreshesAfterInvalidAccessToken(t *testing.T) {}
~~~

The test server must inspect that cgi-bin/token receives appid and secret, msg_sec_check receives normalized content without the secret, and img_sec_check receives a multipart media part. It must also prove the second text check reuses the cached server token.

- [ ] Step 2: Run tests and verify the expected failure.

~~~powershell
D:/bin/go.exe test ./internal/platform/wechat -run 'TestContentSafetyClient' -count=1
~~~

Expected: FAIL because content-safety methods and token cache do not exist.

- [ ] Step 3: Implement the client.

Add a mutex-protected token cache containing token value and expiry. Fetch GET /cgi-bin/token?grant_type=client_credential&appid=...&secret=... and cache for expires_in minus 60 seconds with a minimum positive TTL. For text call POST /wxa/msg_sec_check?access_token=... with JSON content, version 2, scene 2, and openid only when nonempty. For images call POST /wxa/img_sec_check?access_token=... with multipart media and no public URL.

Map errcode 0 to approved, 87014 to rejected, and other errors to a provider error. On invalid access token, invalidate the cache and retry exactly once; do not retry arbitrary failures. Redact query parameters in wrapped errors and never log request bodies, secrets or query tokens.

- [ ] Step 4: Run the HTTP contract tests.

~~~powershell
D:/bin/go.exe test ./internal/platform/wechat -run 'TestContentSafetyClient' -count=1
~~~

Expected: PASS.

- [ ] Step 5: Format and commit.

~~~powershell
gofmt -w internal/platform/wechat/client.go internal/platform/wechat/content_safety.go internal/platform/wechat/content_safety_test.go
D:/bin/go.exe test ./internal/platform/wechat -count=1
git add internal/platform/wechat/client.go internal/platform/wechat/content_safety.go internal/platform/wechat/content_safety_test.go
git commit -m "feat: integrate WeChat content safety APIs"
~~~

### Task 3: Add moderation schema, SQL models, and audit repository

**Files:**
- Create: backend/database/migrations/00013_create_user_moderation.sql
- Create: backend/database/queries/moderation.sql
- Modify: backend/database/queries/leaderboards.sql
- Modify: backend/database/queries/leaderboard_submissions.sql
- Create or modify: backend/internal/store/sqlc/moderation.sql.go
- Modify: backend/database/queries/users.sql
- Modify: backend/internal/store/sqlc/models.go
- Modify: backend/internal/store/sqlc/users.sql.go
- Modify: backend/internal/store/sqlc/leaderboards.sql.go
- Modify: backend/internal/store/sqlc/leaderboard_submissions.sql.go
- Modify: backend/internal/store/sqlc/querier.go
- Create: backend/internal/modules/moderation/repository.go
- Create: backend/internal/modules/moderation/repository_integration_test.go

**Interfaces:**
- Adds moderation fields to db.User, leaderboard rows, and db.UpdateUserProfileParams/db.CreateUserParams.
- Produces SQLAuditStore and ModerationUserStore used by the service, cleanup command, and bootstrap. The exact cleanup interface is `ListUsersForModeration(ctx context.Context, afterID uint64, limit int) ([]db.User, error)` and `UpdateUserModeration(ctx context.Context, arg db.UpdateUserModerationParams) error`; the audit interface is `RecordModerationEvent(ctx context.Context, event moderation.AuditEvent) error`.

- [ ] Step 1: Write schema/repository tests first.

Add a MySQL integration test guarded by GO_SERVICE_TEST_MYSQL_DSN; when unset it calls t.Skip. Assert through repository methods:

~~~go
func TestModerationRepositoryPersistsAuditWithoutRawContent(t *testing.T) {}
func TestUserModerationStatusRoundTrips(t *testing.T) {}
~~~

The audit test inserts a rejection containing a fake sensitive nickname and verifies that the stored row has status/reason/provider request ID but no raw nickname or image bytes. Add a static migration test asserting the migration has Up and Down sections and all three required user fields.

- [ ] Step 2: Run tests and verify the expected failure.

~~~powershell
D:/bin/go.exe test ./internal/modules/moderation -run 'TestModerationRepository|TestUserModerationStatus' -count=1
~~~

Expected: the integration test skips without DSN and compile/static assertions fail because migration and repository methods do not exist.

- [ ] Step 3: Implement the migration and SQL mirrors.

Migration 00013 must add the three user columns and create user_moderation_events with auto-increment ID, user ID, resource type, resource ID, source, moderation status, reason code, provider request ID, created timestamp, user/time and status/time indexes, and a foreign key to users. The Down section drops the event table before removing user columns.

Update users queries and hand-written sqlc mirrors so all user reads scan both statuses and creates/updates can explicitly pass them. Use named fields so existing test literals still compile. Update leaderboard SQL/mirrors to select statuses and include them in GROUP BY; update ranked leaderboard SQL to select them. Implement audit insert and user scan/update helpers in moderation/repository.go.

- [ ] Step 4: Run schema checks and integration tests.

~~~powershell
D:/bin/go.exe test ./internal/modules/moderation ./internal/store/sqlc -count=1
if ($env:GO_SERVICE_TEST_MYSQL_DSN) { goose -dir database/migrations mysql $env:GO_SERVICE_TEST_MYSQL_DSN up }
~~~

Expected: compile/static tests pass; integration tests pass with a disposable MySQL DSN and otherwise report only an explicit skip.

- [ ] Step 5: Format and commit.

~~~powershell
gofmt -w internal/modules/moderation/repository.go internal/modules/moderation/repository_integration_test.go internal/store/sqlc/*.go
D:/bin/go.exe test ./internal/modules/moderation ./internal/store/sqlc -count=1
git add database/migrations/00013_create_user_moderation.sql database/queries/moderation.sql database/queries/leaderboards.sql database/queries/leaderboard_submissions.sql database/queries/users.sql internal/store/sqlc internal/modules/moderation/repository.go internal/modules/moderation/repository_integration_test.go
git commit -m "feat: persist user moderation status and audit events"
~~~

### Task 4: Integrate nickname and avatar moderation into the user module

**Files:**
- Modify: backend/internal/modules/user/dto.go
- Modify: backend/internal/modules/user/store.go
- Modify: backend/internal/modules/user/service.go
- Modify: backend/internal/modules/user/avatar.go
- Modify: backend/internal/modules/user/handler.go
- Modify: backend/internal/modules/user/errors.go
- Modify: backend/internal/modules/user/service_test.go
- Modify: backend/internal/modules/user/handler_test.go

**Interfaces:**
- Adds SetContentModerator(*moderation.Service) to user service.
- Adds internal NicknameModerationStatus and AvatarModerationStatus fields to ProfileResponse with json:"-".
- Adds SafePublicNickname and SafePublicAvatar helpers that expose only approved values.

- [ ] Step 1: Write failing user tests.

Add:

~~~go
func TestUpdateProfileRejectsNicknameWithStableBusinessCode(t *testing.T) {}
func TestGetProfileHidesUnreviewedProfileFromPublicDTO(t *testing.T) {}
func TestUploadAvatarRejectedByModeratorLeavesOldAvatarAndDoesNotSave(t *testing.T) {}
func TestUploadAvatarPendingLeavesOldAvatarAndReturnsNoURL(t *testing.T) {}
func TestUploadAvatarProviderUnavailableDoesNotPublish(t *testing.T) {}
func TestSafePublicProfileHidesNonApprovedValues(t *testing.T) {}
~~~

Use a fake moderator/provider and existing fake store/storage. For rejected/pending cases assert store.updated.ID == 0, storage.saved remains empty, and the original avatar remains unchanged. For nickname PATCH assert response HTTP 400, code NICKNAME_EDIT_DISABLED and no store update.

- [ ] Step 2: Run focused tests and verify failure.

~~~powershell
D:/bin/go.exe test ./internal/modules/user -run 'TestUpdateProfileRejectsNickname|TestGetProfileHides|TestUploadAvatarRejected|TestUploadAvatarPending|TestUploadAvatarProviderUnavailable|TestSafePublicProfile' -count=1
~~~

Expected: FAIL because moderator injection, status fields, safe helpers and business error do not yet exist.

- [ ] Step 3: Implement user integration.

Add the moderation service field and setter. Make ordinary UpdateProfile reject any non-nil nickname before loading/updating the profile with NewBusiness("NICKNAME_EDIT_DISABLED", 400, "昵称暂时无法修改", nil).

For avatar upload, keep the existing file field, size and image decoding checks. Pass processed 256x256 WEBP bytes to the moderator before AvatarStorage.Save. On rejected return AVATAR_REJECTED; on pending return a success payload with moderation_status pending and no URL; on unavailable return HTTP 503 with MODERATION_PROVIDER_UNAVAILABLE. Only approved results call Save and then update the profile with avatar_moderation_status=approved. If database update fails, delete the newly stored file as today. Do not change the old avatar on any pre-save failure.

Set nickname/avatar status fields on all profile writes. GetProfile and public helpers return default nickname/avatar for pending, rejected, unreviewed or empty status; internally keep status fields available to player services but omit them from JSON. Preserve presets and the current-user ownership check for backend avatar URLs.

- [ ] Step 4: Run all user tests.

~~~powershell
D:/bin/go.exe test ./internal/modules/user -count=1
~~~

Expected: PASS, including existing JPG/PNG/WEBP, size, rollback and rate-limit tests. Update only test fixtures that need explicit approved statuses; do not weaken production public filtering for test convenience.

- [ ] Step 5: Format and commit.

~~~powershell
Get-ChildItem internal/modules/user -Filter *.go | ForEach-Object { gofmt -w $_.FullName }
D:/bin/go.exe test ./internal/modules/user -count=1
git add internal/modules/user/dto.go internal/modules/user/store.go internal/modules/user/service.go internal/modules/user/avatar.go internal/modules/user/handler.go internal/modules/user/errors.go internal/modules/user/service_test.go internal/modules/user/handler_test.go
git commit -m "feat: moderate user profiles and avatar uploads"
~~~

### Task 5: Integrate first-authorized WeChat profile synchronization

**Files:**
- Modify: backend/internal/modules/auth/service.go
- Modify: backend/internal/modules/auth/dto.go
- Modify: backend/internal/modules/auth/wechat_test.go
- Modify: backend/internal/modules/auth/service_test.go

**Interfaces:**
- Adds SetNicknameModerator or equivalent moderation dependency to auth service.
- Auth uses openid for the WeChat text check and writes status fields through the existing transactional user store.

- [ ] Step 1: Write failing auth tests.

Add:

~~~go
func TestLoginWithWeChatRejectedFirstNicknameUsesSafeDefault(t *testing.T) {}
func TestLoginWithWeChatProviderFailureDoesNotSaveUnreviewedNickname(t *testing.T) {}
func TestLoginWithWeChatExistingNamedUserCannotBeRenamedByLoginPayload(t *testing.T) {}
func TestLoginWithWeChatDefaultProfileCanReceiveOneControlledApprovedSync(t *testing.T) {}
~~~

Use a fake moderator returning approved/rejected/unavailable and assert created db.CreateUserParams has safe values and statuses. Verify a later login with a non-default saved nickname does not modify it.

- [ ] Step 2: Run tests and verify failure.

~~~powershell
D:/bin/go.exe test ./internal/modules/auth -run 'TestLoginWithWeChat' -count=1
~~~

Expected: FAIL because auth has no moderation dependency and currently synchronizes every supplied nickname.

- [ ] Step 3: Implement auth moderation flow.

Before external user creation, normalize and moderate a supplied nickname when nonempty. A rejected first authorization uses 算术玩家 and nickname_moderation_status=rejected; an unavailable provider uses the safe default and unreviewed/pending according to the decision, without persisting submitted raw value. An approved nickname is saved with approved.

For an existing WeChat user, accept a supplied nickname only when the stored nickname is empty/default or its moderation status is not approved; after an approved controlled sync, subsequent login payloads cannot rename the user. Existing external avatar synchronization continues to accept WeChat HTTPS hosts, but writes avatar_moderation_status and never makes non-approved URLs public.

Keep login functional when privacy authorization sends empty nickname/avatar. Map provider outage to a safe profile and a structured log; do not return AppSecret or provider payloads.

- [ ] Step 4: Run auth and user suites.

~~~powershell
D:/bin/go.exe test ./internal/modules/auth ./internal/modules/user -count=1
~~~

Expected: PASS.

- [ ] Step 5: Format and commit.

~~~powershell
Get-ChildItem internal/modules/auth -Filter *.go | ForEach-Object { gofmt -w $_.FullName }
D:/bin/go.exe test ./internal/modules/auth ./internal/modules/user -count=1
git add internal/modules/auth/service.go internal/modules/auth/dto.go internal/modules/auth/wechat_test.go internal/modules/auth/service_test.go
git commit -m "feat: moderate WeChat profile synchronization"
~~~

### Task 6: Enforce safe public data in player, rank, and friend flows

**Files:**
- Modify: backend/internal/modules/player/leaderboard.go
- Modify: backend/internal/modules/player/leaderboard_query.go
- Modify: backend/internal/modules/player/rank.go
- Modify: backend/internal/modules/player/rank_history.go
- Modify: backend/internal/modules/player/friend_room.go
- Modify: backend/internal/modules/player/matchmaking.go
- Modify: backend/internal/modules/player/friend_history.go
- Modify: backend/internal/modules/player/leaderboard_test.go
- Modify: backend/internal/modules/player/matchmaking_test.go
- Create: backend/internal/modules/player/public_profile_test.go

**Interfaces:**
- Every public player response uses user.SafePublicNickname and user.SafePublicAvatar with row/profile status fields.
- Redis room snapshots are sanitized at creation and again at response time, including bot/human opponent cards and progress responses.

- [ ] Step 1: Write failing public-output tests.

Add:

~~~go
func TestLeaderboardHidesRejectedAndUnreviewedProfiles(t *testing.T) {}
func TestRankLeaderboardHidesPendingAvatarAndNickname(t *testing.T) {}
func TestFriendRoomResponseReFiltersPersistedUnsafeSnapshot(t *testing.T) {}
func TestRankAndFriendHistoryHideUnsafeOpponentNames(t *testing.T) {}
~~~

Build fake rows with valid-looking but non-approved names/avatars and assert each response contains 算术玩家 and sun, never the stored values.

- [ ] Step 2: Run tests and verify failure.

~~~powershell
D:/bin/go.exe test ./internal/modules/player -run 'TestLeaderboardHides|TestRankLeaderboardHides|TestFriendRoomResponseReFilters|TestRankAndFriendHistoryHides' -count=1
~~~

Expected: FAIL because current paths only normalize syntax and do not use moderation status.

- [ ] Step 3: Implement safe output at every boundary.

Update SQL row types/queries to carry status fields. Replace player-local normalizers with user safe helpers. When a profile is inserted into a room or matchmaking ticket, use safe values; when an existing Redis room/ticket is returned, sanitize values again. Sanitize the opponent side of rank history and friend history as well as the current user. Do not expose statuses, reason codes or provider IDs.

Do not alter scores, rank calculations, room membership, question contracts, bot behavior or existing response field names. Keep the bot display name as a safe server-owned value.

- [ ] Step 4: Run player tests.

~~~powershell
D:/bin/go.exe test ./internal/modules/player -count=1
~~~

Expected: PASS.

- [ ] Step 5: Format and commit.

~~~powershell
Get-ChildItem internal/modules/player -Filter *.go | ForEach-Object { gofmt -w $_.FullName }
D:/bin/go.exe test ./internal/modules/player -count=1
git add internal/modules/player/leaderboard.go internal/modules/player/leaderboard_query.go internal/modules/player/rank.go internal/modules/player/rank_history.go internal/modules/player/friend_room.go internal/modules/player/matchmaking.go internal/modules/player/friend_history.go internal/modules/player/leaderboard_test.go internal/modules/player/matchmaking_test.go internal/modules/player/public_profile_test.go
git commit -m "feat: filter moderation state from public player data"
~~~

### Task 7: Wire production dependencies, config, and idempotent history cleanup

**Files:**
- Create: backend/cmd/moderation-cleanup/main.go
- Create: backend/internal/modules/moderation/cleanup.go
- Create: backend/internal/modules/moderation/cleanup_test.go
- Modify: backend/internal/app/bootstrap.go
- Modify: backend/internal/config/config.go
- Modify: backend/internal/config/loader.go
- Modify: backend/internal/config/validate.go
- Modify: backend/.env.example
- Modify: backend/configs/config.example.yaml
- Modify: backend/deployments/production.env.example
- Modify: backend/README.md
- Modify: backend/docs/production-native.md

**Interfaces:**
- API bootstrap constructs one wechat.Client, one moderation service, one audit repository, and injects them into auth/user services.
- cmd/moderation-cleanup supports --dry-run, scans users, updates only moderation/profile fields, and reports counters without rewards or global cache deletion.

- [ ] Step 1: Write failing wiring and cleanup tests.

Add:

~~~go
func TestCleanupDryRunDoesNotUpdateOrCreateRewards(t *testing.T) {}
func TestCleanupIsIdempotentForAlreadyApprovedProfile(t *testing.T) {}
func TestProductionConfigKeepsModerationTimeoutWithinBounds(t *testing.T) {}
~~~

The cleanup fake store contains approved, rejected and unreviewed rows. Two non-dry runs yield the same second-run update count and no player progress/coin calls. Config tests assert default timeout and retry bounds.

- [ ] Step 2: Run tests and verify failure.

~~~powershell
D:/bin/go.exe test ./internal/modules/moderation ./internal/config -run 'TestCleanup|TestProductionConfigKeepsModeration' -count=1
~~~

Expected: FAIL because cleanup service, config fields and bootstrap wiring do not exist.

- [ ] Step 3: Implement wiring and cleanup.

Add ModerationConfig with timeout and maximum retry settings, bind GO_SERVICE_MODERATION_TIMEOUT and GO_SERVICE_MODERATION_MAX_RETRIES, default to 5s and 1, and validate timeout 1–30s/retries 0–2. Instantiate wechat.Client once in bootstrap, construct the SQL audit store, and inject the moderation service into auth/user.

Implement cleanup with explicit --dry-run. For each user, moderate current nickname and any stored external avatar only through the configured provider. Approved values get approved status; rejected nicknames become 算术玩家 and rejected avatars are replaced by the safe preset; provider failure leaves the value hidden/unreviewed. Record an audit event for each decision. Never mutate player progress, award coins, delete accounts/history, or call FLUSHDB. Since current public caches are not independent, report cache count zero and rely on response-time filtering.

- [ ] Step 4: Run config, cleanup and compile tests.

~~~powershell
D:/bin/go.exe test ./internal/modules/moderation ./internal/config ./cmd/moderation-cleanup -count=1
D:/bin/go.exe build ./cmd/moderation-cleanup ./cmd/api
~~~

Expected: PASS/build success. MySQL/Redis integration remains environment-dependent and must be reported explicitly.

- [ ] Step 5: Format and commit.

~~~powershell
Get-ChildItem cmd/moderation-cleanup,internal/modules/moderation,internal/app,internal/config -Recurse -Filter *.go | ForEach-Object { gofmt -w $_.FullName }
D:/bin/go.exe test ./internal/modules/moderation ./internal/config ./cmd/moderation-cleanup -count=1
git add cmd/moderation-cleanup internal/modules/moderation internal/app/bootstrap.go internal/config .env.example configs/config.example.yaml deployments/production.env.example README.md docs/production-native.md
git commit -m "feat: wire moderation and add history cleanup command"
~~~

### Task 8: Complete route, security, deployment documentation and full verification

**Files:**
- Modify: backend/docs/openapi.yaml
- Modify: backend/docs/backend-completion.md
- Modify: backend/docs/release-acceptance.md
- Create: backend/internal/modules/moderation/routes_security_test.go

**Interfaces:**
- Documents unchanged user endpoints and moderation outcomes; no frontend change is required.
- Produces final verification evidence required before integration/push.

- [ ] Step 1: Write failing route/security tests.

Add route tests asserting:

~~~go
func TestAvatarRouteStillRequiresBearerAndMultipartFileField(t *testing.T) {}
func TestModerationErrorsUseCodeMessageDataEnvelope(t *testing.T) {}
func TestNoBackendSourceContainsKnownSecretAssignments(t *testing.T) {}
~~~

The source scan covers only tracked backend source and config examples, permits placeholder names, and rejects literal JWT/AppSecret/token values. It must fail if a new response or source path leaks sensitive data.

- [ ] Step 2: Run tests and verify expected failure.

~~~powershell
D:/bin/go.exe test ./internal/modules/moderation -run 'TestAvatarRoute|TestModerationErrors|TestNoBackendSource' -count=1
~~~

Expected: FAIL until route/security fixtures and final response behavior are wired.

- [ ] Step 3: Update docs and fix route/security gaps.

Document auth/wechat-login with empty or moderated profile data; GET/PATCH users/me with NICKNAME_EDIT_DISABLED; POST users/me/avatar multipart field file and approved/rejected/pending responses; public filtering; cleanup dry-run/apply commands; required production variables; migration 00013 and API restart. Keep examples free of credentials and ensure upload never accepts another user ID.

- [ ] Step 4: Run complete verification from D:/微信小游戏/backend.

~~~powershell
Get-ChildItem -Recurse -Filter *.go | ForEach-Object { gofmt -l $_.FullName }
D:/bin/go.exe test -count=1 ./...
D:/bin/go.exe vet ./...
D:/bin/go.exe build ./...
git diff --check
~~~

Expected: gofmt -l prints no files; test, vet and build exit 0; diff check prints no errors. If an integration test is skipped, report the exact missing DSN rather than claiming it passed.

- [ ] Step 5: Review final diff and commit only backend changes.

~~~powershell
git status --short
git diff --stat -- backend
git diff --name-only -- backend
~~~

Verify no frontend or Godot path is staged. From D:/微信小游戏, stage only the exact backend paths changed by Tasks 1–8 (the files listed in those tasks), never `git add .` or `git add backend` because the worktree contains unrelated user changes. Run the full suite once more after corrections, then commit:

~~~powershell
git add backend/internal/modules/moderation/types.go backend/internal/modules/moderation/normalize.go backend/internal/modules/moderation/service.go backend/internal/modules/moderation/service_test.go backend/internal/modules/moderation/repository.go backend/internal/modules/moderation/repository_integration_test.go backend/internal/modules/moderation/cleanup.go backend/internal/modules/moderation/cleanup_test.go backend/internal/modules/moderation/routes_security_test.go backend/internal/platform/wechat/content_safety.go backend/internal/platform/wechat/content_safety_test.go backend/internal/platform/wechat/client.go backend/internal/apperror/errors.go backend/internal/http/response/error.go backend/internal/http/response/error_test.go backend/database/migrations/00013_create_user_moderation.sql backend/database/queries/moderation.sql backend/database/queries/leaderboards.sql backend/database/queries/leaderboard_submissions.sql backend/database/queries/users.sql backend/internal/store/sqlc/models.go backend/internal/store/sqlc/users.sql.go backend/internal/store/sqlc/leaderboards.sql.go backend/internal/store/sqlc/leaderboard_submissions.sql.go backend/internal/store/sqlc/querier.go backend/internal/store/sqlc/moderation.sql.go backend/internal/modules/user/dto.go backend/internal/modules/user/store.go backend/internal/modules/user/service.go backend/internal/modules/user/avatar.go backend/internal/modules/user/handler.go backend/internal/modules/user/errors.go backend/internal/modules/user/service_test.go backend/internal/modules/user/handler_test.go backend/internal/modules/auth/service.go backend/internal/modules/auth/dto.go backend/internal/modules/auth/wechat_test.go backend/internal/modules/auth/service_test.go backend/internal/modules/player/leaderboard.go backend/internal/modules/player/leaderboard_query.go backend/internal/modules/player/rank.go backend/internal/modules/player/rank_history.go backend/internal/modules/player/friend_room.go backend/internal/modules/player/matchmaking.go backend/internal/modules/player/friend_history.go backend/internal/modules/player/leaderboard_test.go backend/internal/modules/player/matchmaking_test.go backend/internal/modules/player/public_profile_test.go backend/cmd/moderation-cleanup/main.go backend/internal/app/bootstrap.go backend/internal/config/config.go backend/internal/config/loader.go backend/internal/config/validate.go backend/.env.example backend/configs/config.example.yaml backend/deployments/production.env.example backend/README.md backend/docs/production-native.md backend/deployments/production.env.example backend/docs/openapi.yaml backend/docs/backend-completion.md backend/docs/release-acceptance.md
git diff --cached --name-only
git commit -m "feat: add server-side profile content moderation"
~~~

Do not push automatically. Before deployment, apply migration 00013, set production moderation/WeChat variables, run cleanup in dry-run mode, inspect counts, then run it without --dry-run and restart the API.
