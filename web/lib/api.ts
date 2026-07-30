import {
  clearTokens,
  getAccessToken,
  getRefreshToken,
  setTokens,
} from "./auth-storage";

const base = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8080/api/v1";

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
      throw new ApiError(text || "服务器返回了无法识别的数据", response.status);
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

async function refreshAccessToken(): Promise<string | null> {
  const refreshToken = getRefreshToken();
  if (!refreshToken) return null;

  const response = await fetch(`${base}/users/refreshToken`, {
    method: "POST",
    headers: { Authorization: `Bearer ${refreshToken}` },
  });
  if (!response.ok) {
    clearTokens();
    return null;
  }

  const token = response.headers.get("x-jwt-token");
  if (!token) {
    clearTokens();
    return null;
  }
  setTokens(token, refreshToken);
  return token;
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

  if (options.auth) {
    const token = getAccessToken();
    if (token) headers.set("Authorization", `Bearer ${token}`);
  }

  let response = await fetch(base + path, {
    ...init,
    headers,
    cache: init.cache ?? "no-store",
  });

  if (
    response.status === 401 &&
    options.auth &&
    options.retry !== false &&
    typeof window !== "undefined"
  ) {
    const token = await refreshAccessToken();
    if (token) {
      headers.set("Authorization", `Bearer ${token}`);
      response = await fetch(base + path, {
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
  const response = await fetch(`${base}/users/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });
  await parseResult<null>(response);

  const accessToken = response.headers.get("x-jwt-token");
  const refreshToken = response.headers.get("x-refresh-token");
  if (!accessToken || !refreshToken) {
    throw new ApiError("登录成功，但服务器未返回登录令牌", response.status);
  }
  setTokens(accessToken, refreshToken);
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
    clearTokens();
  }
}
