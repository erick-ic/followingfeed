"use client";

import { useEffect, useState, type MouseEvent } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { LoaderCircle, Menu, PenLine, UserRound, X } from "lucide-react";
import { useAuth } from "./auth-provider";
import { ThemeToggle } from "./theme-toggle";

export function SiteHeader() {
  const pathname = usePathname();
  const { ready, authenticated } = useAuth();
  const [open, setOpen] = useState(false);
  const [openingEditor, setOpeningEditor] = useState(false);

  const publicLinks = [{ href: "/", label: "最新文章" }];
  const privateLinks = [{ href: "/dashboard/articles", label: "我的文章" }];
  const links = authenticated ? [...publicLinks, ...privateLinks] : publicLinks;

  useEffect(() => {
    setOpeningEditor(false);
  }, [pathname]);

  function markEditorOpening(event: MouseEvent<HTMLAnchorElement>) {
    if (
      pathname === "/articles/new" ||
      event.button !== 0 ||
      event.metaKey ||
      event.ctrlKey ||
      event.shiftKey ||
      event.altKey
    ) {
      return;
    }
    setOpeningEditor(true);
  }

  return (
    <header className="site-header">
      <div className="header-inner">
        <Link href="/" className="brand" onClick={() => setOpen(false)}>
          <span>Following</span>
          <span className="brand-accent">Feed</span>
        </Link>

        <nav className="desktop-nav" aria-label="主导航">
          {links.map((link) => (
            <Link
              key={link.href}
              href={link.href}
              className={
                pathname === link.href ? "nav-link active" : "nav-link"
              }
            >
              {link.label}
            </Link>
          ))}
        </nav>

        <div className="header-actions">
          <ThemeToggle />
          {ready &&
            (authenticated ? (
              <>
                <Link
                  href="/articles/new"
                  className="header-cta"
                  aria-busy={openingEditor}
                  onClick={markEditorOpening}
                >
                  {openingEditor ? (
                    <LoaderCircle className="spin" size={15} />
                  ) : (
                    <PenLine size={15} />
                  )}
                  {openingEditor ? "正在打开" : "写文章"}
                </Link>
                <Link
                  href="/profile"
                  className={
                    pathname === "/profile"
                      ? "icon-button desktop-only active"
                      : "icon-button desktop-only"
                  }
                  aria-label="个人信息"
                  title="个人信息"
                >
                  <UserRound size={17} />
                </Link>
              </>
            ) : (
              <div className="auth-links desktop-only">
                <Link href="/login" className="nav-link">
                  登录
                </Link>
                <Link href="/register" className="header-cta">
                  注册
                </Link>
              </div>
            ))}
          <button
            type="button"
            className="icon-button mobile-menu-button"
            aria-expanded={open}
            aria-label={open ? "关闭菜单" : "打开菜单"}
            onClick={() => setOpen((value) => !value)}
          >
            {open ? <X size={19} /> : <Menu size={19} />}
          </button>
        </div>
      </div>

      {open && (
        <nav className="mobile-nav" aria-label="移动端导航">
          {links.map((link) => (
            <Link
              key={link.href}
              href={link.href}
              className={
                pathname === link.href ? "nav-link active" : "nav-link"
              }
              onClick={() => setOpen(false)}
            >
              {link.label}
            </Link>
          ))}
          {authenticated ? (
            <Link
              href="/profile"
              className={
                pathname === "/profile" ? "nav-link active" : "nav-link"
              }
              onClick={() => setOpen(false)}
            >
              <UserRound size={16} />
              个人信息
            </Link>
          ) : (
            <>
              <Link
                href="/login"
                className="nav-link"
                onClick={() => setOpen(false)}
              >
                登录
              </Link>
              <Link
                href="/register"
                className="nav-link"
                onClick={() => setOpen(false)}
              >
                注册
              </Link>
            </>
          )}
        </nav>
      )}
    </header>
  );
}
