import { defineConfig, globalIgnores } from "eslint/config";
import nextCoreWebVitals from "eslint-config-next/core-web-vitals";

export default defineConfig([
  ...nextCoreWebVitals,
  {
    rules: {
      // 当前客户端页面使用 effect 发起异步请求，
      // 并同步维护 loading/error 状态。
      "react-hooks/set-state-in-effect": "off",
    },
  },
  globalIgnores([".next/**"]),
]);
