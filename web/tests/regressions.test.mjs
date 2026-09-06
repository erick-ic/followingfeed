import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import { createRequire } from "node:module";
import ts from "typescript";

const require = createRequire(import.meta.url);
const root = path.resolve(import.meta.dirname, "..");
function load(entry, mocks = {}, globals = {}) {
  const cache = new Map();
  function visit(filename) {
    if (cache.has(filename)) return cache.get(filename);
    const exports = {};
    cache.set(filename, exports);
    const sandbox = {
      exports,
      Error,
      Number,
      Headers,
      AbortController,
      setTimeout,
      clearTimeout,
      process: { env: {} },
      ...globals,
      require(id) {
        if (id in mocks) return mocks[id];
        if (id.startsWith(".")) {
          const target = path.resolve(path.dirname(filename), id);
          return visit([target, target + ".ts", target + ".tsx"].find((f) => fs.existsSync(f)));
        }
        return require(id);
      },
    };
    const code = ts.transpileModule(fs.readFileSync(filename, "utf8"), {
      compilerOptions: {
        module: ts.ModuleKind.CommonJS,
        jsx: ts.JsxEmit.ReactJSX,
        target: ts.ScriptTarget.ES2022,
      },
    }).outputText;
    vm.runInNewContext(code, sandbox, { filename });
    return exports;
  }
  return visit(path.join(root, entry));
}
const uiMocks = {
  "next/link": { default: "a" },
  "lucide-react": new Proxy({}, { get: () => "span" }),
  ...Object.fromEntries(
    [
      "interaction-summary",
      "page-select",
      "article-link",
      "list-scroll-restorer",
      "local-date-time",
    ].map((n) => ["../components/" + n, new Proxy({}, { get: () => "span" })]),
  ),
};
function home(api) {
  return load("app/page.tsx", {
    ...uiMocks,
    "../lib/api": { api },
    "next/navigation": {
      redirect(url) {
        throw new Error("redirect:" + url);
      },
    },
  }).default;
}

test("empty out-of-range page makes one request and redirects to last page", async () => {
  let count = 0;
  const page = home(async () => {
    count++;
    return { items: [], totalPages: 0 };
  });
  await assert.rejects(
    page({ searchParams: Promise.resolve({ page: "25" }) }),
    /redirect:\/\?page=1/,
  );
  assert.equal(count, 1);
});
test("invalid and unbounded pages never reach API", async () => {
  for (const value of ["Infinity", "NaN", "1.5", "-1", "0", "1001", "9007199254740992"]) {
    let count = 0;
    const page = home(async () => {
      count++;
    });
    await assert.rejects(page({ searchParams: Promise.resolve({ page: value }) }), /redirect:\//);
    assert.equal(count, 0);
  }
});
test("API failure renders an error without redirect loop", async () => {
  const result = await home(async () => {
    throw new Error("offline");
  })({ searchParams: Promise.resolve({ page: "2" }) });
  assert.match(JSON.stringify(result), /offline/);
});
test("deleting last page redirects directly using returned totalPages", async () => {
  await assert.rejects(
    home(async () => ({ items: [], totalPages: 3 }))({
      searchParams: Promise.resolve({ page: "8" }),
    }),
    /redirect:\/\?page=3/,
  );
});

test("timeout also covers a stalled response body and does not retry writes", async () => {
  let calls = 0;
  const { api } = load(
    "lib/api.ts",
    {},
    {
      setTimeout: (fn) => setTimeout(fn, 10),
      fetch: async (_url, init) => {
        calls++;
        return {
          status: 200,
          ok: true,
          headers: new Headers(),
          text: () =>
            new Promise((_resolve, reject) =>
              init.signal.addEventListener("abort", () => reject(new Error("aborted"))),
            ),
        };
      },
    },
  );
  await assert.rejects(api("/articles/publish", { method: "POST" }), /请求超时/);
  assert.equal(calls, 1);
});
test("caller cancellation is preserved", async () => {
  const { api } = load(
    "lib/api.ts",
    {},
    {
      fetch: async (_url, init) => {
        if (init.signal.aborted) throw new Error("aborted");
      },
    },
  );
  const controller = new AbortController();
  controller.abort();
  await assert.rejects(api("/pub/list", { signal: controller.signal }), /请求已取消/);
});
test("failed logout keeps credentials so the user can retry", async () => {
  let token = "existing-token";
  const { logoutRequest } = load(
    "lib/api.ts",
    {
      "./auth-storage": {
        getAccessToken: () => token,
        clearAccessToken: () => {
          token = null;
        },
      },
    },
    {
      fetch: async () => ({
        status: 503,
        ok: false,
        headers: new Headers(),
        text: async () => '{"msg":"unavailable"}',
      }),
    },
  );
  await assert.rejects(logoutRequest(), /unavailable/);
  assert.equal(token, "existing-token");
});

test("article not found is distinct from upstream failure", async () => {
  const { ApiError } = load("lib/api.ts");
  const mocks = {
    "next/link": { default: "a" },
    "lucide-react": new Proxy({}, { get: () => "span" }),
    "next/navigation": {
      notFound() {
        throw new Error("NEXT_NOT_FOUND");
      },
    },
    ...Object.fromEntries(
      [
        "markdown-content",
        "follow-button",
        "like-button",
        "collect-button",
        "interaction-summary",
        "back-button",
        "scroll-actions",
        "detail-scroll-top",
        "local-date-time",
      ].map((n) => ["../../../components/" + n, new Proxy({}, { get: () => "span" })]),
    ),
  };
  let requests = 0;
  const notFoundPage = load("app/articles/[id]/page.tsx", {
    ...mocks,
    "../../../lib/api": {
      ApiError,
      api: async () => {
        requests++;
        throw new ApiError("gone", 404);
      },
    },
  }).default;
  await assert.rejects(notFoundPage({ params: Promise.resolve({ id: "1" }) }), /NEXT_NOT_FOUND/);
  await assert.rejects(
    notFoundPage({ params: Promise.resolve({ id: "invalid" }) }),
    /NEXT_NOT_FOUND/,
  );
  assert.equal(requests, 1);
  const failedPage = load("app/articles/[id]/page.tsx", {
    ...mocks,
    "../../../lib/api": {
      ApiError,
      api: async () => {
        throw new ApiError("unavailable", 503);
      },
    },
  }).default;
  assert.match(
    JSON.stringify(await failedPage({ params: Promise.resolve({ id: "1" }) })),
    /文章暂时无法加载/,
  );
});
