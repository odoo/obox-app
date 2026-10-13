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
} from "../../wailsjs/go/main/App";
import { update } from "../../wailsjs/go/models";
import { EventsOn } from "../../wailsjs/runtime/runtime";

// Matches backend State constants in internal/update/types.go
export type UpdateState =
  | "idle"
  | "failed"
  | "update_available"
  | "checking"
  | "downloading"
  | "installing";

const MIN_CHECK_DURATION = 600; // ms minimum duration to prevent rapid state flickering

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

interface ProgressData {
  state?: UpdateState;
  downloaded: number;
  total: number;
  percent?: number;
}

export interface UpdateContextType {
  data: {
    status: UpdateState;
    info: update.Info | null;
    error: string;
    progress: number;
    bannerDismissed: boolean;
    // Helper accessors
    checking: boolean;
    downloading: boolean;
    applying: boolean;
    isLatest: boolean;
    checkFailed: boolean;
    checkError: string;
    failed: boolean;
    errorMessage: string;
  };
  setters: {
    setInfo: (info: update.Info | null) => void;
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

export const UpdateContext = createContext<UpdateContextType | null>(null);

export function useUpdate() {
  return useContext(UpdateContext);
}

export function getFriendlyErrorMessage(err: string): string {
  if (!err) {
    return "We couldn't check for updates. Please check your internet connection and try again.";
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
    return "The update check took too long. Please try again.";
  }

  if (lower.includes("stalled")) {
    return "The update download was interrupted. Please check your internet connection and try again.";
  }

  if (lower.includes("forbidden") || lower.includes("403")) {
    return "We couldn't access the update service right now. Please try again later.";
  }

  if (lower.includes("no compatible asset") || lower.includes("no asset")) {
    return "No update is available for your device.";
  }

  return "We couldn't check for updates. Please try again later.";
}

interface UpdateContextWrapperProps {
  children: ReactNode;
}

export const UpdateContextWrapper = ({ children }: UpdateContextWrapperProps) => {
  const [status, setStatus] = useState<UpdateState>("checking");
  const [info, setInfo] = useState<update.Info | null>(null);
  const [error, setError] = useState<string>("");
  const [progress, setProgress] = useState(0);
  const [bannerDismissed, setBannerDismissed] = useState(true);

  const inFlightRef = useRef(false);

  const checkForUpdate = useCallback(
    async () => {
      if (inFlightRef.current) {
        return;
      }

      inFlightRef.current = true;
      setStatus("checking");
      setError("");

      const startTime = Date.now();

      const finish = async (newStatus: UpdateState, newError: string = "") => {
        const elapsed = Date.now() - startTime;
        if (elapsed < MIN_CHECK_DURATION) {
          await sleep(MIN_CHECK_DURATION - elapsed);
        }
        setStatus(newStatus);
        setError(newError);
        inFlightRef.current = false;
      };

      // Fast offline pre-check with smooth minimum duration to avoid flickering
      if (typeof navigator !== "undefined" && !navigator.onLine) {
        const offlineMsg = "Cannot reach update server. Please check your internet connection.";
        setInfo((prev) => ({
          state: "failed",
          error: offlineMsg,
          currentVersion: prev?.currentVersion || "unknown",
          latestVersion: prev?.latestVersion || "unknown",
          notes: prev?.notes || "",
          seen: false,
        }));
        await finish("failed", offlineMsg);
        return;
      }

      try {
        const result = await CheckForUpdate();
        if (!result) {
          const defaultErr = "Failed to check for updates.";
          setInfo((prev) => ({
            notes: "",
            seen: false,
            currentVersion: prev?.currentVersion || "unknown",
            latestVersion: prev?.latestVersion || "unknown",
            state: "failed",
            error: defaultErr,
          }));
          await finish("failed", defaultErr);
          return;
        }

        // Check if backend returned failed state or error message
        if (result.state === "failed" || Boolean(result.error)) {
          const errMsg = result.error || "Failed to check for updates.";
          const friendlyErr = getFriendlyErrorMessage(errMsg);
          setInfo({
            ...result,
            state: "failed",
            error: friendlyErr,
          });
          await finish("failed", friendlyErr);
          return;
        }

        if (result.state === "idle") {
          setInfo(result);
          await finish("idle", "");
          return;
        }

        if (result.state === "update_available") {
          setBannerDismissed(result.seen);
          setInfo(result);
          await finish("update_available", "");
          return;
        }

        // Fallback for any other state
        setInfo(result);
        await finish(result.state as UpdateState, result.error || "");
      } catch (err: any) {
        console.error("Failed to check for updates:", err);
        const rawErr = typeof err === "string" ? err : err?.message || String(err) || "Could not check for updates.";
        const friendly = getFriendlyErrorMessage(rawErr);
        setInfo((prev) => ({
          state: "failed",
          error: friendly,
          currentVersion: prev?.currentVersion || "unknown",
          latestVersion: prev?.latestVersion || "unknown",
          notes: prev?.notes || "",
          seen: false,
        }));
        await finish("failed", friendly);
      }
    },
    []
  );

  // Abort stalled downloads immediately when connection drops
  useEffect(() => {
    const handleOffline = () => {
      if (inFlightRef.current) {
        CancelUpdate().catch(() => { });
        inFlightRef.current = false;
        setStatus("failed");
        setError("Network connection lost during download. Please check your internet connection.");
      }
    };

    window.addEventListener("offline", handleOffline);
    return () => {
      window.removeEventListener("offline", handleOffline);
    };
  }, []);

  useEffect(() => {
    checkForUpdate()

    // 3. Download progress events from backend
    const unsubscribeProgress = EventsOn("update-progress", (data: ProgressData) => {
      if (!data) return;
      if (data.state) {
        setStatus(data.state);
      }
      if (typeof data.percent === "number") {
        setProgress(data.percent);
      } else if (data.total > 0) {
        setProgress(Math.round((data.downloaded / data.total) * 100));
      }
    });

    return () => {
      unsubscribeProgress();
    };
  }, []);

  const handleUpdate = useCallback(async () => {
    if (inFlightRef.current) {
      return;
    }
    if (typeof navigator !== "undefined" && !navigator.onLine) {
      setStatus("failed");
      setError("No internet connection available. Please check your network and try again.");
      return;
    }

    inFlightRef.current = true;
    setStatus("downloading");
    setError("");
    setProgress(0);

    try {
      await DownloadUpdate();
    } catch (err: any) {
      inFlightRef.current = false;
      const rawMsg = typeof err === "string" ? err : err?.message || String(err) || "Update failed. Please try again.";
      const msg = getFriendlyErrorMessage(rawMsg);
      setStatus("failed");
      setError(msg);
      setInfo((prev) =>
        prev
          ? { ...prev, state: "failed", error: msg }
          : {
              state: "failed",
              error: msg,
              currentVersion: "unknown",
              latestVersion: "unknown",
              notes: "",
              seen: false,
            }
      );
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
    if (info && info.state === "update_available") {
      setStatus("update_available");
    } else {
      setStatus("idle");
    }
  }, [info]);

  const handleDismiss = useCallback(async () => {
    if (info?.latestVersion) {
      await MarkUpdateSeen(info.latestVersion).catch(() => { });
    }
    setBannerDismissed(true);
  }, [info]);

  const data = {
    status,
    info,
    error,
    progress,
    bannerDismissed,
    // Helper flags derived from status
    checking: status === "checking",
    downloading: status === "downloading",
    applying: status === "installing",
    isLatest: status === "idle",
    checkFailed: status === "failed",
    checkError: error,
    failed: status === "failed",
    errorMessage: error,
  };

  const setters = {
    setInfo,
    setBannerDismissed,
    setStatus,
  };

  const actions = {
    checkForUpdate,
    handleUpdate,
    handleCancel,
    handleDismiss,
  };

  return (
    <UpdateContext.Provider value={{ data, setters, actions }}>
      {children}
    </UpdateContext.Provider>
  );
};
