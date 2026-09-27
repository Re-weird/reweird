"use client";

import { Suspense, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { AlertTriangle, ArrowLeft, Clock, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ApiError, githubApi } from "@/lib/api";
import { useAppState } from "@/lib/app-state";
import { refreshAccountActivity } from "@/lib/account-activity";
import { takeGitHubReturn } from "@/lib/github-return";

// GitHub's App "Setup URL" points here. It arrives with installation_id,
// an OAuth code (the App must request user authorization during install),
// and the state ReWeird signed; the API verifies all three.
function GitHubCallback() {
  const params = useSearchParams();
  const router = useRouter();
  const { setShowNewProject } = useAppState();
  const [state, setState] = useState<"working" | "pending" | "error">("working");
  const [message, setMessage] = useState("");
  const started = useRef(false);

  useEffect(() => {
    if (started.current) return;
    started.current = true;
    const installationID = Number(params.get("installation_id"));
    if (params.get("setup_action") === "request") {
      setState("pending");
      return;
    }
    if (params.get("setup_action") === "update" && !params.get("code")) {
      // Repository access changed on GitHub for an existing connection.
      refreshAccountActivity();
      router.replace(takeGitHubReturn().path);
      return;
    }
    if (!installationID && !params.get("code")) {
      setState("error");
      setMessage(params.get("error_description") ?? "GitHub didn't send an authorization code. Start again from Settings.");
      return;
    }
    // Without installation_id this is the authorize step: the API looks up
    // the user's existing installation, or asks for an install.
    githubApi.connect({ installation_id: installationID || 0, code: params.get("code") ?? "", state: params.get("state") ?? "" })
      .then(() => {
        refreshAccountActivity();
        const destination = takeGitHubReturn();
        router.replace(destination.path);
        if (destination.reopenNewProject) setShowNewProject(true);
      })
      .catch(async (cause) => {
        if (cause instanceof ApiError && cause.code === "INSTALL_REQUIRED") {
          const status = await githubApi.status().catch(() => null);
          if (status?.install_url) { window.location.assign(status.install_url); return; }
        }
        setState("error");
        setMessage(cause instanceof Error ? cause.message : "GitHub couldn't be connected.");
      });
  }, [params, router, setShowNewProject]);

  if (state === "working") {
    return <p className="flex items-center gap-2 text-sm text-muted-foreground"><RefreshCw className="size-4 animate-spin" strokeWidth={1.5} /> Connecting GitHub…</p>;
  }
  return (
    <div className="flex max-w-md flex-col items-start gap-4">
      <span className="grid size-11 place-items-center rounded-lg bg-surface-2 text-muted-foreground ring-1 ring-border">
        {state === "pending" ? <Clock className="size-5" strokeWidth={1.5} /> : <AlertTriangle className="size-5" strokeWidth={1.5} />}
      </span>
      <div>
        <h1 className="text-lg font-semibold text-foreground">{state === "pending" ? "Waiting for an organization owner" : "GitHub wasn't connected"}</h1>
        <p className="mt-1 text-sm leading-relaxed text-muted-foreground">
          {state === "pending" ? "GitHub sent the install request to an owner of that organization. Once they approve it, connect again from Settings." : message}
        </p>
      </div>
      <Button asChild variant="outline" size="sm"><Link href="/settings"><ArrowLeft /> Back to Settings</Link></Button>
    </div>
  );
}

export default function GitHubCallbackPage() {
  return <div data-tw className="py-16"><Suspense><GitHubCallback /></Suspense></div>;
}
