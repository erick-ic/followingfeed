//go:build e2e

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"followingfeed/config"
	ijwt "followingfeed/internal/handler/jwt"
	"followingfeed/internal/observability"
	"followingfeed/internal/repository/dao"
	"followingfeed/ioc"
	"followingfeed/migrations"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type e2eResult struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

type e2eFixture struct {
	server     *gin.Engine
	db         *gorm.DB
	redis      redis.Cmdable
	emails     []string
	userIDs    []int64
	articleIDs []int64
}

func newE2EFixture(t *testing.T) *e2eFixture {
	t.Helper()
	cfg, err := config.Load()
	require.NoError(t, err)
	metrics := observability.NewMetrics()
	db, dbCleanup, err := ioc.InitMySQL(cfg, metrics)
	require.NoError(t, err)
	t.Cleanup(dbCleanup)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, migrations.ApplyAll(context.Background(), sqlDB))
	redisClient, err := ioc.InitRedis(cfg, metrics)
	require.NoError(t, err)
	app, appCleanup, err := InitApp(cfg)
	require.NoError(t, err)
	t.Cleanup(appCleanup)
	f := &e2eFixture{server: app.Server, db: db, redis: redisClient}
	t.Cleanup(func() { f.cleanup(t) })
	return f
}

func (f *e2eFixture) signupAndLogin(t *testing.T, nickname string) (int64, string) {
	t.Helper()
	email := fmt.Sprintf("e2e-%s@example.com", uuid.NewString())
	f.emails = append(f.emails, email)
	res, _ := f.request(t, http.MethodPost, "/api/v1/users/signup", map[string]any{
		"nickname": nickname, "email": email, "password": "Passw0rd!", "confirm_password": "Passw0rd!",
	}, "")
	require.Equal(t, 0, res.Code, res.Msg)
	var user dao.User
	require.NoError(t, f.db.Where("email = ?", email).First(&user).Error)
	uid := int64(user.Id)
	f.userIDs = append(f.userIDs, uid)
	res, recorder := f.request(t, http.MethodPost, "/api/v1/users/login", map[string]any{
		"email": email, "password": "Passw0rd!",
	}, "")
	require.Equal(t, 0, res.Code, res.Msg)
	token := recorder.Header().Get("X-JWT-Token")
	require.NotEmpty(t, token)
	return uid, token
}

func (f *e2eFixture) publish(t *testing.T, token, title string) int64 {
	t.Helper()
	res, _ := f.request(t, http.MethodPost, "/api/v1/articles/publish", map[string]any{
		"id": 0, "title": title, "content": "这是一篇用于验证完整 HTTP 发布链路的端到端测试文章。",
	}, token)
	require.Equal(t, 0, res.Code, res.Msg)
	var id int64
	require.NoError(t, json.Unmarshal(res.Data, &id))
	require.Positive(t, id)
	f.articleIDs = append(f.articleIDs, id)
	return id
}

func (f *e2eFixture) request(
	t *testing.T,
	method, path string,
	body any,
	token string,
) (e2eResult, *httptest.ResponseRecorder) {
	t.Helper()
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		require.NoError(t, err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	f.server.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var result e2eResult
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &result), recorder.Body.String())
	return result, recorder
}

func (f *e2eFixture) cleanup(t *testing.T) {
	t.Helper()
	if len(f.articleIDs) > 0 {
		require.NoError(t, f.db.Where("biz_id IN ?", f.articleIDs).Delete(&dao.UserLikeBiz{}).Error)
		require.NoError(
			t,
			f.db.Where("biz_id IN ?", f.articleIDs).Delete(&dao.UserCollectionBiz{}).Error,
		)
		require.NoError(t, f.db.Where("biz_id IN ?", f.articleIDs).Delete(&dao.Interactive{}).Error)
		require.NoError(t, f.db.Where("id IN ?", f.articleIDs).Delete(&dao.PublishArticle{}).Error)
		require.NoError(t, f.db.Where("id IN ?", f.articleIDs).Delete(&dao.Article{}).Error)
	}
	if len(f.userIDs) > 0 {
		require.NoError(
			t,
			f.db.Where("follower_id IN ? OR following_id IN ?", f.userIDs, f.userIDs).
				Delete(&dao.Follow{}).
				Error,
		)
		require.NoError(t, f.db.Where("id IN ?", f.userIDs).Delete(&dao.User{}).Error)
		keys := make([]string, 0, len(f.userIDs)+1)
		for _, uid := range f.userIDs {
			keys = append(keys, fmt.Sprintf("article:first_page:%d", uid))
		}
		keys = append(keys, "ip-limiter:192.0.2.1")
		require.NoError(t, f.redis.Del(context.Background(), keys...).Err())
	}
}

func TestHTTPUserSignupLoginProfile(t *testing.T) {
	f := newE2EFixture(t)
	uid, token := f.signupAndLogin(t, "认证用户")
	res, _ := f.request(t, http.MethodGet, "/api/v1/users/profile", nil, token)
	require.Equal(t, 0, res.Code, res.Msg)
	var profile struct {
		ID       int64  `json:"id"`
		Nickname string `json:"nickname"`
	}
	require.NoError(t, json.Unmarshal(res.Data, &profile))
	assert.Equal(t, uid, profile.ID)
	assert.Equal(t, "认证用户", profile.Nickname)
}

func TestHTTPArticlePublishAndPublicDetail(t *testing.T) {
	f := newE2EFixture(t)
	uid, token := f.signupAndLogin(t, "文章作者")
	articleID := f.publish(t, token, "端到端发布文章")
	res, _ := f.request(t, http.MethodGet, fmt.Sprintf("/api/v1/pub/detail/%d", articleID), nil, "")
	require.Equal(t, 0, res.Code, res.Msg)
	var article struct {
		ID       int64  `json:"id"`
		Title    string `json:"title"`
		AuthorID int64  `json:"authorId"`
	}
	require.NoError(t, json.Unmarshal(res.Data, &article))
	assert.Equal(t, articleID, article.ID)
	assert.Equal(t, uid, article.AuthorID)
	assert.Equal(t, "端到端发布文章", article.Title)
}

func TestHTTPFollowAndFeed(t *testing.T) {
	f := newE2EFixture(t)
	authorID, authorToken := f.signupAndLogin(t, "订阅作者")
	articleID := f.publish(t, authorToken, "关注流文章")
	_, readerToken := f.signupAndLogin(t, "订阅读者")
	res, _ := f.request(
		t,
		http.MethodPost,
		fmt.Sprintf("/api/v1/users/%d/follow", authorID),
		nil,
		readerToken,
	)
	require.Equal(t, 0, res.Code, res.Msg)
	res, _ = f.request(t, http.MethodGet, "/api/v1/feed?page=1&pageSize=10", nil, readerToken)
	require.Equal(t, 0, res.Code, res.Msg)
	var page struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal(res.Data, &page))
	require.NotEmpty(t, page.Items)
	assert.Equal(t, articleID, page.Items[0].ID)
}

func TestHTTPInteractionLikeCollectAndCollections(t *testing.T) {
	f := newE2EFixture(t)
	_, token := f.signupAndLogin(t, "互动用户")
	articleID := f.publish(t, token, "互动测试文章")
	res, _ := f.request(
		t,
		http.MethodPost,
		fmt.Sprintf("/api/v1/pub/articles/%d/like", articleID),
		nil,
		token,
	)
	require.Equal(t, 0, res.Code, res.Msg)
	res, _ = f.request(
		t,
		http.MethodPost,
		fmt.Sprintf("/api/v1/pub/articles/%d/collect", articleID),
		nil,
		token,
	)
	require.Equal(t, 0, res.Code, res.Msg)
	res, _ = f.request(
		t,
		http.MethodGet,
		fmt.Sprintf("/api/v1/pub/articles/%d/interactions/status", articleID),
		nil,
		token,
	)
	require.Equal(t, 0, res.Code, res.Msg)
	var status struct {
		Liked     bool `json:"liked"`
		Collected bool `json:"collected"`
	}
	require.NoError(t, json.Unmarshal(res.Data, &status))
	assert.True(t, status.Liked)
	assert.True(t, status.Collected)
	res, _ = f.request(
		t,
		http.MethodGet,
		"/api/v1/users/me/collections?page=1&pageSize=10",
		nil,
		token,
	)
	require.Equal(t, 0, res.Code, res.Msg)
	var page struct {
		Total int64 `json:"total"`
	}
	require.NoError(t, json.Unmarshal(res.Data, &page))
	assert.Equal(t, int64(1), page.Total)
}

func TestHTTPSessionRevocationAndLoss(t *testing.T) {
	f := newE2EFixture(t)
	_, token := f.signupAndLogin(t, "会话测试")
	cfg, err := config.Load()
	require.NoError(t, err)
	jwtHandler := ijwt.NewRedisJWTHandler(f.redis, cfg)
	claims, err := jwtHandler.ParseAccessToken(token)
	require.NoError(t, err)
	require.NoError(t, jwtHandler.CheckSession(context.Background(), claims.Ssid))
	f.request(t, http.MethodPost, "/api/v1/users/logout", nil, token)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/profile", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	f.server.ServeHTTP(resp, req)
	require.Equal(t, http.StatusUnauthorized, resp.Code)

	// 用真实 Redis 删除一个会话模拟数据丢失；只影响本测试令牌。
	_, token = f.signupAndLogin(t, "丢失测试")
	claims, err = jwtHandler.ParseAccessToken(token)
	require.NoError(t, err)
	require.NoError(t, f.redis.Del(context.Background(), "users:session:v2:"+claims.Ssid).Err())
	req = httptest.NewRequest(http.MethodGet, "/api/v1/users/profile", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp = httptest.NewRecorder()
	f.server.ServeHTTP(resp, req)
	require.Equal(t, http.StatusUnauthorized, resp.Code)
}
