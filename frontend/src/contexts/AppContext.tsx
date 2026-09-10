import { createContext, useEffect, useState } from "react";
import { main } from "../../wailsjs/go/models";
import { AppVariable } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";

const RETRY_INTERVAL = 5000;

type AppContextType = {
  setters: {};
  data: {
    os: string | null;
    app: main.AppVariable | null;
    appId: string;
    ipAddress: string;
    isWindows: boolean;
    isMac: boolean;
    isLinux: boolean;
    serverIsRunning: boolean;
  };
  actions: {};
};

export const AppContext = createContext({} as AppContextType);

interface AppContextWrapper {
  children: React.ReactNode;
}

export const AppContextWrapper = ({ children }: AppContextWrapper) => {
  const [app, setApp] = useState<main.AppVariable | null>(null);

  const os = app?.os || null;
  const data = {
    app,
    os,
    appId: app?.appId || "",
    ipAddress: app?.ipAddress || "",
    isWindows: os === "windows",
    isMac: os === "darwin",
    isLinux: os === "linux",
    serverIsRunning: app?.serverRunning ?? false,
  };
  const setters = {};
  const actions = {};

  useEffect(() => {
    let cancelled = false;
    let retryId: number | null = null;

    const fetchAppContext = async () => {
      try {
        const variables = await AppVariable();
        if (cancelled) {
          return;
        }

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
