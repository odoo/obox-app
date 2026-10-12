import { createContext, useContext, useEffect, useState } from "react";
import { main } from "../../wailsjs/go/models";
import { ApiAppVariable } from "../api/client";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { GetSessionToken } from "../../wailsjs/go/main/App";
import { setWailsToken } from "../api/authState";
import { backendService } from "../services/backend";

export function detectWails(): boolean {
  return (
    typeof window !== "undefined" &&
    !!(window as unknown as Record<string, unknown>)["go"]
  );
}

const RETRY_INTERVAL = 5000;

export type AppContextType = {
  setters: Record<string, never>;
  data: {
    os: string | null;
    isWails: boolean;
    ready: boolean;
    serverURL: string | null;
    app: (main.AppVariable & { kioskMode?: boolean }) | ApiAppVariable | null;
    appId: string;
    ipAddress: string;
    isWindows: boolean;
    isMac: boolean;
    isLinux: boolean;
    isKioskMode: boolean;
    serverIsRunning: boolean;
    networkPrintingEnabled: boolean;
    autoStart: boolean;
    isLoading: boolean;
  };
  actions: Record<string, never>;
};

export const AppContext = createContext({} as AppContextType);

export function useApp() {
  return useContext(AppContext);
}

interface AppContextWrapperProps {
  children: React.ReactNode;
}

export const AppContextWrapper = ({ children }: AppContextWrapperProps) => {
  const isWails = detectWails();
  const [ready, setReady] = useState(!isWails);
  const [app, setApp] = useState<(main.AppVariable & { kioskMode?: boolean }) | ApiAppVariable | null>(null);
  const [serverURL, setServerURL] = useState<string | null>(
    isWails ? null : typeof window !== "undefined" ? window.location.origin : null,
  );

  const os = app?.os || null;
  const data = {
    isWails,
    ready,
    serverURL,
    app,
    os,
    appId: app?.appId || "",
    ipAddress: app?.ipAddress || "",
    isWindows: os === "windows",
    isMac: os === "darwin",
    isLinux: os === "linux",
    isKioskMode: Boolean(app?.kioskMode),
    serverIsRunning: app?.serverRunning ?? false,
    autoStart: app?.autoStart ?? false,
    networkPrintingEnabled: app?.networkPrintingEnabled ?? false,
    isLoading: app === null,
  };
  const setters = {} as Record<string, never>;
  const actions = {} as Record<string, never>;

  useEffect(() => {
    if (!isWails) return;

    GetSessionToken()
      .then((token: string) => {
        if (token) {
          setWailsToken(token);
        }
        setReady(true);
      })
      .catch((err: unknown) => {
        console.error("Failed to get Wails session token:", err);
        setReady(true);
      });
  }, [isWails]);

  useEffect(() => {
    let cancelled = false;
    let retryId: number | null = null;

    const fetchAppContext = async () => {
      try {
        const variables = await backendService.getAppVariable();

        if (cancelled) return;
        setApp(variables);
      } catch (error) {
        console.error("Failed to fetch app context:", error);
        if (!cancelled) {
          retryId = window.setTimeout(fetchAppContext, RETRY_INTERVAL);
        }
      }
    };

    const unsubscribe = EventsOn("app:variables_changed", (variables: main.AppVariable) => {
      setApp(variables);
    });

    fetchAppContext();

    return () => {
      cancelled = true;
      if (retryId !== null) {
        clearTimeout(retryId);
      }
      if (unsubscribe) {
        unsubscribe();
      }
    };
  }, []);

  return (
    <AppContext.Provider value={{ data, setters, actions }}>
      {children}
    </AppContext.Provider>
  );
};
