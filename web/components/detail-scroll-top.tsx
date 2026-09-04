"use client";

import { useLayoutEffect } from "react";

/** 进入详情路由时始终滚动到页面顶部，不继承上一个路由的滚动位置。 */
export function DetailScrollTop() {
  useLayoutEffect(() => {
    window.scrollTo({ top: 0, left: 0, behavior: "auto" });
  }, []);

  return null;
}
