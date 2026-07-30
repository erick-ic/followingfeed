export const defaultNicknames = [
  "云端旅人",
  "像素松鼠",
  "代码诗人",
  "开源捕手",
  "异步漫游者",
  "接口侦探",
  "终端守夜人",
  "数据拾光者",
  "分支园丁",
  "容器船长",
] as const;

export function randomNickname() {
  return defaultNicknames[Math.floor(Math.random() * defaultNicknames.length)];
}
