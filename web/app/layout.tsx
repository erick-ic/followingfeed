import type { Metadata } from "next";
import "./globals.css";
import { Providers } from "./providers";
import { SiteHeader } from "../components/site-header";
import { TopLoader } from "../components/top-loader";

export const metadata: Metadata = {
  title: {
    default: "FollowingFeed · 技术内容社区",
    template: "%s · FollowingFeed",
  },
  description: "发现、创作并分享值得持续关注的技术内容。",
};

const themeScript = `
  try {
    const saved = localStorage.getItem("followingfeed_theme");
    const dark = saved === "dark" || (!saved && matchMedia("(prefers-color-scheme: dark)").matches);
    document.documentElement.classList.toggle("dark", dark);
  } catch {}
`;

export default function Layout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="zh-CN" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeScript }} />
      </head>
      <body>
        <TopLoader />
        <Providers>
          <SiteHeader />
          <main className="site-main">{children}</main>
          <footer className="site-footer">
            <div className="footer-inner">
              <span className="footer-brand">
                Following<span>Feed</span>
              </span>
              <span>记录技术，也关注创造技术的人。</span>
              <span>© {new Date().getFullYear()}</span>
            </div>
          </footer>
        </Providers>
      </body>
    </html>
  );
}
