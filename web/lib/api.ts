import { clearAccessToken, getAccessToken, setAccessToken } from "./auth-storage";
import type { Follow, PageResult } from "./types";
import type { UserProfile } from "./types";

const publicBase = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8080/api/v1";

// 浏览器通过宿主机端口访问 API；容器内服务端渲染通过 Compose 服务名访问。
function apiBase() {
  if (typeof window === "undefined") {
    return process.env.INTERNAL_API_BASE || publicBase;
  }
  return publicBase;
}
let currentProfileRequest: Promise<UserProfile> | null = null;
export type PublicInteractions = {
  likeCount?: number;
  readCount?: number;
  collectCount?: number;
};

export type InteractionStatus = PublicInteractions & {
  liked: boolean;
  collected: boolean;
};

const interactionStatusRequests = new Map<number, Promise<InteractionStatus>>();

export function getCurrentProfile() {
  if (!currentProfileRequest) {
    currentProfileRequest = api<UserProfile>("/users/profile", {}, { auth: true }).catch(
      (error) => {
        currentProfileRequest = null;
        throw error;
      },
    );
  }
  return currentProfileRequest;
}

export function clearCurrentProfileCache() {
  currentProfileRequest = null;
}

export function clearInteractionStatus(articleId: number) {
  interactionStatusRequests.delete(articleId);
}

// 文章详情中的多个互动控件共享同一次请求；完成后移除，避免跨登录态复用个人状态。
export function getInteractionStatus(articleId: number) {
  const existing = interactionStatusRequests.get(articleId);
  if (existing) return existing;
  const request = api<InteractionStatus>(
    `/pub/articles/${articleId}/interactions/status`,
    {},
    { auth: true },
  ).finally(() => interactionStatusRequests.delete(articleId));
  interactionStatusRequests.set(articleId, request);
  return request;
}

export type ApiResult<T> = {
  code: number;
  msg: string;
  data: T;
};

export class ApiError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly code?: number,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

// fetch 在断网、DNS 失败等情况下会抛出浏览器自带的英文 TypeError。
// 统一在请求层转换为中文错误，避免各页面直接展示 “Failed to fetch”。
async function request(input: string, init?: RequestInit): Promise<Response> {
  try {
    // include 允许跨端口开发环境接收和发送后端设置的 HttpOnly 刷新 Cookie。
    return await fetch(input, { credentials: "include", ...init });
  } catch {
    throw new ApiError("网络连接失败，请检查网络后重试", 0);
  }
}

type ApiOptions = {
  auth?: boolean;
  retry?: boolean;
};

async function parseResult<T>(response: Response): Promise<ApiResult<T>> {
  const text = await response.text();
  let body: ApiResult<T> | null = null;

  if (text) {
    try {
      body = JSON.parse(text) as ApiResult<T>;
    } catch {
      // 反向代理或网关可能返回英文 HTML；不要把原始内容直接展示给用户。
      throw new ApiError("服务器返回了无法识别的数据", response.status);
    }
  }

  if (!response.ok) {
    throw new ApiError(body?.msg || "请求失败，请稍后重试", response.status);
  }
  if (!body) {
    throw new ApiError("服务器未返回数据", response.status);
  }
  if (body.code !== 0) {
    throw new ApiError(body.msg || "操作失败", response.status, body.code);
  }
  return body;
}

let refreshRequest: Promise<string | null> | null = null;
let refreshDisabled = false;

function clearAuthentication() {
  clearAccessToken();
  refreshDisabled = true;
}

async function requestNewAccessToken(): Promise<string | null> {
  if (refreshDisabled || typeof window === "undefined") return null;

  const response = await request(`${apiBase()}/users/refreshToken`, {
    method: "POST",
  });
  if (response.status === 401) {
    clearAuthentication();
    return null;
  }
  if (!response.ok) await parseResult<null>(response);

  const token = response.headers.get("x-jwt-token");
  if (!token) {
    clearAuthentication();
    return null;
  }
  setAccessToken(token);
  return token;
}

async function refreshAccessToken(): Promise<string | null> {
  if (!refreshRequest) {
    refreshRequest = requestNewAccessToken().finally(() => {
      refreshRequest = null;
    });
  }
  return refreshRequest;
}

export async function api<T>(
  path: string,
  init: RequestInit = {},
  options: ApiOptions = {},
): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  let attemptedRefresh = false;
  if (options.auth) {
    let token = getAccessToken();
    if (!token && typeof window !== "undefined") {
      token = await refreshAccessToken();
      attemptedRefresh = true;
    }
    if (token) headers.set("Authorization", `Bearer ${token}`);
  }

  let response = await request(apiBase() + path, {
    ...init,
    headers,
    cache: init.cache ?? "no-store",
  });

  if (
    response.status === 401 &&
    options.auth &&
    !attemptedRefresh &&
    options.retry !== false &&
    typeof window !== "undefined"
  ) {
    const token = await refreshAccessToken();
    if (token) {
      headers.set("Authorization", `Bearer ${token}`);
      response = await request(apiBase() + path, {
        ...init,
        headers,
        cache: "no-store",
      });
    }
  }

  const body = await parseResult<T>(response);
  return body.data;
}

export async function loginRequest(email: string, password: string) {
  const response = await request(`${apiBase()}/users/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });
  await parseResult<null>(response);

  const accessToken = response.headers.get("x-jwt-token");
  if (!accessToken) {
    throw new ApiError("登录成功，但服务器未返回登录令牌", response.status);
  }
  refreshDisabled = false;
  setAccessToken(accessToken);
}

export async function registerRequest(
  nickname: string,
  email: string,
  password: string,
  confirmPassword: string,
) {
  await api<null>("/users/signup", {
    method: "POST",
    body: JSON.stringify({
      nickname,
      email,
      password,
      confirm_password: confirmPassword,
    }),
  });
}

export async function logoutRequest() {
  try {
    await api<null>("/users/logout", { method: "POST" }, { auth: true });
  } finally {
    clearAuthentication();
  }
}

export async function followUser(userId: number) {
  return api<null>(`/users/${userId}/follow`, { method: "POST" }, { auth: true });
}

export async function unfollowUser(userId: number) {
  return api<null>(`/users/${userId}/unfollow`, { method: "POST" }, { auth: true });
}

export async function getFollowStatus(userId: number) {
  return api<{ following: boolean }>(`/users/${userId}/following/status`, {}, { auth: true });
}

export async function getFollowing(userId: number, page = 1) {
  return api<PageResult<Follow>>(
    `/users/${userId}/following?page=${page}&pageSize=10`,
    {},
    { auth: true },
  );
}

export async function getFollowers(userId: number, page = 1) {
  return api<PageResult<Follow>>(
    `/users/${userId}/followers?page=${page}&pageSize=10`,
    {},
    { auth: true },
  );
}
