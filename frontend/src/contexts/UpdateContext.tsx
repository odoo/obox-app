import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import {
  CancelUpdate,
  CheckForUpdate,
  DownloadUpdate,
  MarkUpdateSeen,
} from "@bindings/obox-app/app";
import * as update from "@bindings/obox-app/internal/update/models";
import { State } from "@bindings/github.com/wailsapp/wails/v3/pkg/updater/models";
import { onEvent } from "../lib/wails";
import { AppContext } from "./AppContext";

// update.Info is now a proper class from the update namespace

// Matches backend State constants in github.com/wailsapp/wails/v3/pkg/updater
export type UpdateState =
  | "unconfigured"
  | "idle"
  | "checking"
  | "up-to-date"
  | "available"
  | "downloading"
  | "verifying"
  | "installing"
  | "ready"
  | "error";

const MIN_CHECK_DURATION = 600; // ms minimum duration to prevent rapid state flickering

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

interface ProgressData {
  written?: number;
  downloaded?: number;
  total: number;
  rate?: number;
}

export interface UpdateContextType {
  data: {
    info: update.Info;
    progress: number;
  };
  setters: {
    setInfo: (info: update.Info) => void;
    setBannerDismissed: (dismissed: boolean) => void;
    setStatus: (status: UpdateState) => void;
  };
  actions: {
    checkForUpdate: () => Promise<void>;
    handleUpdate: () => Promise<void>;
    handleCancel: () => Promise<void>;
    handleDismiss: () => Promise<void>;
  };
}

export const UpdateContext = createContext<UpdateContextType>({} as UpdateContextType);

export function useUpdate() {
  return useContext(UpdateContext);
}

export function getFriendlyErrorMessage(err?: string, context: "check" | "download" | "install" = "check"): string {
  const defaultMsg =
    context === "download"
      ? "We couldn't download the update. Please check your internet connection and try again."
      : context === "install"
        ? "Failed to install update. Please try again."
        : "We couldn't check for updates. Please check your internet connection and try again.";

  if (!err) {
    return defaultMsg;
  }

  const lower = err.toLowerCase();

  if (lower.includes("rate limit")) {
    return "Too many update requests were made. Please try again later.";
  }

  if (
    lower.includes("offline") ||
    lower.includes("no such host") ||
    lower.includes("connection refused") ||
    lower.includes("network is unreachable") ||
    lower.includes("no route to host")
  ) {
    return "We couldn't connect to the update service. Please check your internet connection and try again.";
  }

  if (lower.includes("timeout") || lower.includes("deadline")) {
    return context === "download"
      ? "The update download timed out. Please try again."
      : "The update check took too long. Please try again.";
  }

  if (lower.includes("stalled")) {
    return "The update download was interrupted. Please check your internet connection and try again.";
  }

  if (lower.includes("checksum")) {
    return "Update file verification failed. Please try downloading again.";
  }

  if (lower.includes("forbidden") || lower.includes("403")) {
    return "We couldn't access the update service right now. Please try again later.";
  }

  if (lower.includes("no compatible asset") || lower.includes("no asset")) {
    return "No update is available for your device.";
  }

  if (
    lower.includes("not writable") ||
    lower.includes("permission denied") ||
    lower.includes("access denied") ||
    lower.includes("access is denied")
  ) {
    return "We couldn't install the update (permission denied). Please download and install the update manually.";
  }

  return defaultMsg;
}

interface UpdateContextWrapperProps {
  children: ReactNode;
}

export const UpdateContextWrapper = ({ children }: UpdateContextWrapperProps) => {
  const appContext = useContext(AppContext);
  const version = appContext.data.build.version || "dev";
  const [info, setInfo] = useState<update.Info>(new update.Info({
    state: "checking" as State,
    error: "",
    currentVersion: version,
    latestVersion: version,
    notes: "",
    seen: true,
  }));

  const [progress, setProgress] = useState(0);

  const inFlightRef = useRef(false);
  const hasCheckedStartupRef = useRef(false);
  const infoRef = useRef(info);
  useEffect(() => {
    infoRef.current = info;
  }, [info]);

  const checkForUpdate = useCallback(async () => {
    if (inFlightRef.current) {
      return;
    }

    inFlightRef.current = true;
    setProgress(0);
    setInfo((prev) => ({ ...prev, state: "checking" as State, error: "" }));

    const startTime = Date.now();

    const finish = async (newInfo: update.Info) => {
      const elapsed = Date.now() - startTime;
      if (elapsed < MIN_CHECK_DURATION) {
        await sleep(MIN_CHECK_DURATION - elapsed);
      }
      setInfo(newInfo);
      inFlightRef.current = false;
    };

    if (typeof navigator !== "undefined" && !navigator.onLine) {
      const offlineMsg = "Cannot reach update server. Please check your internet connection.";
      await finish({ ...info, state: "error" as State, error: offlineMsg });
      return;
    }

    try {
      const result = await CheckForUpdate();
      if (!result) {
        await finish({ ...info, state: "error" as State, error: "Failed to check for updates." });
        return;
      }

      if (result.state === "error") {
        await finish({ ...result, error: getFriendlyErrorMessage(result.error) });
        return;
      }

      await finish(result);
    } catch (err: unknown) {
      console.error("Failed to check for updates:", err);
      const rawErr = typeof err === "string" ? err : (err instanceof Error ? err.message : String(err)) || "Could not check for updates.";
      await finish({ ...info, state: "error" as State, error: getFriendlyErrorMessage(rawErr) });
    }
  }, [info]);

  // Abort stalled downloads immediately when connection drops
  useEffect(() => {
    const handleOffline = () => {
      if (infoRef.current?.state === "downloading") {
        CancelUpdate().catch(() => { });
        inFlightRef.current = false;
        setInfo((prev) => ({
          ...prev,
          state: "error" as State,
          error: "Network connection lost during download. Please check your internet connection.",
        }));
      }
    };

    window.addEventListener("offline", handleOffline);
    return () => {
      window.removeEventListener("offline", handleOffline);
    };
  }, []);

  // Listen for backend progress and status events
  useEffect(() => {
    const unsubscribeProgress = onEvent<ProgressData>("update-progress", (data) => {
      if (!data) return;
      const bytes = data.written ?? data.downloaded ?? 0;
      if (data.total > 0) {
        setProgress(Math.min(Math.round((bytes / data.total) * 100), 100));
      }
    });

    const unsubscribeStatus = onEvent<update.Info>("update-status", (status) => {
      if (!status) return;
      setInfo((prev) => {
        if (!prev) return prev;
        let nextError = prev.error;
        if (status.error) {
          nextError = getFriendlyErrorMessage(status.error, "install");
        }
        return { ...prev, state: status.state, error: nextError };
      });
      if (status.state === "error" || status.state === "available" || status.state === "ready" || status.state === "idle" || status.state === "up-to-date") {
        inFlightRef.current = false;
      }
    });

    return () => {
      unsubscribeProgress();
      unsubscribeStatus();
    };
  }, []);

  // Automatic startup check (skipped in dev mode)
  const isAppLoading = appContext.data.isLoading;
  const build = appContext.data.build;
  const isDev = build.dev ?? false;
  const appVersion = build.version || "dev";

  useEffect(() => {
    if (hasCheckedStartupRef.current || isAppLoading) {
      return;
    }
    hasCheckedStartupRef.current = true;

    if (isDev) {
      setInfo(new update.Info({
        state: "idle" as State,
        error: "",
        currentVersion: appVersion,
        latestVersion: appVersion,
        notes: "",
        seen: true,
      }));
      return;
    }

    checkForUpdate();
  }, [isAppLoading, isDev, appVersion, checkForUpdate]);

  const handleUpdate = useCallback(async () => {
    if (inFlightRef.current) {
      return;
    }
    if (typeof navigator !== "undefined" && !navigator.onLine) {
      setInfo((prev) => ({ ...prev, state: "error" as State, error: "No internet connection available. Please check your network and try again." }));
      return;
    }

    inFlightRef.current = true;
    setProgress(0);
    setInfo((prev) => (prev ? { ...prev, state: "downloading" as State, error: "" } : prev));

    try {
      await DownloadUpdate();
    } catch (err: unknown) {
      inFlightRef.current = false;
      const rawMsg = typeof err === "string" ? err : (err instanceof Error ? err.message : String(err)) || "Update failed. Please try again.";
      if (rawMsg.includes("canceled") || rawMsg.includes("cancelled")) {
        setInfo((prev) => (prev ? { ...prev, state: "available" as State, error: "" } : prev));
        return;
      }
      setInfo((prev) => ({ ...prev, state: "error" as State, error: getFriendlyErrorMessage(rawMsg, "download") }));
    }
  }, []);

  const handleCancel = useCallback(async () => {
    try {
      await CancelUpdate().catch(() => { });
    } catch (err) {
      console.error("Failed to cancel update:", err);
    }
    inFlightRef.current = false;
    setProgress(0);
    setInfo((prev) => {
      if (!prev) return prev;
      const isNewerAvailable = !!prev.latestVersion && prev.latestVersion !== prev.currentVersion;
      return {
        ...prev,
        state: (isNewerAvailable ? "available" : "idle") as State,
      };
    });
  }, []);

  const handleDismiss = useCallback(async () => {
    if (info.latestVersion) {
      await MarkUpdateSeen(info.latestVersion).catch(() => { });
    }
    setInfo((prev) => (prev ? { ...prev, seen: true } : prev));
  }, [info]);

  const data = { info, progress };

  const setters = {
    setInfo,
    setBannerDismissed: (seen: boolean) => setInfo((prev) => (prev ? { ...prev, seen } : prev)),
    setStatus: (state: UpdateState) => setInfo((prev) => (prev ? { ...prev, state: state as State } : prev)),
  };

  const actions = { checkForUpdate, handleUpdate, handleCancel, handleDismiss };

  return (
    <UpdateContext.Provider value={{ data, setters, actions }}>
      {children}
    </UpdateContext.Provider>
  );
};
