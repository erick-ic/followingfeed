import Link from "next/link";
import { api } from "../../lib/api";
import RegistrationForm from "../../components/registration-form";

export const dynamic = "force-dynamic";

export default async function RegisterPage() {
  let enabled = false;
  let message = "目前暂未开放注册，请稍后再来。";
  try {
    const site = await api<{ signupEnabled: boolean }>("/site");
    enabled = site.signupEnabled;
  } catch {
    message = "暂时无法连接注册服务，请稍后重试。";
  }
  if (enabled) return <RegistrationForm />;
  return (
    <div className="form-shell">
      <div className="form-panel">
        <h1>注册暂不可用</h1>
        <p className="form-intro">{message}</p>
        <Link href="/" className="button secondary small">
          返回首页
        </Link>
      </div>
    </div>
  );
}
