"use client";

import { GitHubSettingsSection } from "../github-connection";
import { SettingsStatusView } from "../settings-status";

export default function SettingsPage() {
  return <>
    <GitHubSettingsSection />
    <SettingsStatusView />
  </>;
}
