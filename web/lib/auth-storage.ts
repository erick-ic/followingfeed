// 访问令牌只保存在当前页面进程的内存中，避免被持久化到 localStorage。
// 页面刷新后由 HttpOnly 刷新 Cookie 静默换取新的访问令牌。
let accessToken: string | null = null;

// 清理旧版本持久化的令牌；迁移完成后不会再向 Web Storage 写入认证信息。
if (typeof window !== "undefined") {
  window.localStorage.removeItem("followingfeed_access_token");
  window.localStorage.removeItem("followingfeed_refresh_token");
}

export function getAccessToken() {
  return accessToken;
}

export function setAccessToken(token: string) {
  accessToken = token;
}

export function clearAccessToken() {
  accessToken = null;
}
