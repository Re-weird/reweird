"use client";

import { useRouter } from "next/navigation";
import { AccountDashboardView } from "./account-dashboard";

export default function AccountPage() {
  const router = useRouter();
  return <AccountDashboardView onNavigate={(view) => router.push(view === "projects" ? "/projects" : "/settings")} />;
}
