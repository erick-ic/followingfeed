import { redirect } from "next/navigation";

export default function LegacyUserPage() {
  redirect("/profile");
}
