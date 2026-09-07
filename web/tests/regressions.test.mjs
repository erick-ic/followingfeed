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

// Minimal hook runner: preserves state and dependency semantics across user-driven renders.
function retryPage(entry, prefix, api) {
  const slots = [];
  let cursor = 0;
  let effects = [];
  const changed = (old, next) => !old || next.some((value, i) => !Object.is(value, old[i]));
  const react = {
    useState(initial) {
      const index = cursor++;
      if (!(index in slots)) slots[index] = initial;
      return [
        slots[index],
        (value) => {
          slots[index] = typeof value === "function" ? value(slots[index]) : value;
        },
      ];
    },
    useCallback(fn, deps) {
      const index = cursor++;
      if (changed(slots[index]?.deps, deps)) slots[index] = { fn, deps };
      return slots[index].fn;
    },
    useEffect(fn, deps) {
      const index = cursor++;
      if (changed(slots[index], deps)) effects.push(fn);
      slots[index] = deps;
    },
  };
  const mocks = {
    react,
    "next/link": { default: "a" },
    "lucide-react": new Proxy({}, { get: () => "span" }),
    [prefix + "lib/api"]: { api },
  };
  for (const name of [
    "auth-guard",
    "confirm-dialog",
    "interaction-summary",
    "page-select",
    "local-date-time",
    "article-link",
    "list-scroll-restorer",
  ]) {
    mocks[prefix + "components/" + name] = new Proxy({}, { get: () => name });
  }
  const Page = load(entry, mocks).default;
  const Component = Page().props.children.type;
  return () => {
    cursor = 0;
    effects = [];
    const tree = Component();
    effects.forEach((fn) => fn());
    return tree;
  };
}
function findElement(node, predicate) {
  if (!node || typeof node !== "object") return;
  if (Array.isArray(node)) {
    for (const item of node) {
      const found = findElement(item, predicate);
      if (found) return found;
    }
    return;
  }
  if (predicate(node)) return node;
  return findElement(node.props?.children, predicate);
}
const settleRequests = async () => {
  for (let i = 0; i < 10; i++) await Promise.resolve();
};
for (const [name, entry, prefix] of [
  ["feed", "app/feed/page.tsx", "../../"],
  ["my articles", "app/dashboard/articles/page.tsx", "../../../"],
]) {
  for (const failedPage of [1, 2]) {
    test(`${name} retries failed page ${failedPage} without resetting pagination`, async () => {
      const calls = [];
      let failures = 0;
      const render = retryPage(entry, prefix, async (url) => {
        calls.push(url);
        const page = Number(new URL(url, "http://test").searchParams.get("page"));
        if (page === failedPage && failures++ < 2) throw new Error("请求超时，请稍后重试");
        const list = {
          items: [{ id: 1, title: "恢复后的文章", status: 2 }],
          page,
          totalPages: 2,
          total: 2,
        };
        return name === "feed" ? list : { list, summary: { draft: 0, published: 2 } };
      });
      render();
      await settleRequests();
      let tree = render();
      if (failedPage === 2) {
        const select = findElement(tree, (n) => n.type === "page-select");
        assert.ok(select);
        select.props.onChange(2);
        render();
        await settleRequests();
        tree = render();
      }
      for (let retry = 0; retry < 2; retry++) {
        assert.match(JSON.stringify(tree), /请求超时/);
        assert.doesNotMatch(JSON.stringify(tree), /还没有文章/);
        const button = findElement(
          tree,
          (n) => n.type === "button" && n.props.children === "重新加载",
        );
        assert.ok(button);
        const before = calls.length;
        button.props.onClick();
        tree = render();
        assert.match(JSON.stringify(tree), /正在加载/);
        await settleRequests();
        tree = render();
        assert.equal(calls.length, before + 1);
        assert.match(calls.at(-1), new RegExp(`page=${failedPage}&`));
      }
      assert.match(JSON.stringify(tree), /恢复后的文章/);
      assert.doesNotMatch(JSON.stringify(tree), /请求超时/);
    });
  }
}
