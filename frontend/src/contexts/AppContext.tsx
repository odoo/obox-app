import { createContext, useCallback, useEffect, useState } from "react";
import * as main from "@bindings/obox-app/models";
import { AppVariable } from "@bindings/obox-app/app";
import { onEvent } from "../lib/wails";

const RETRY_INTERVAL = 5000;

type AppContextType = {
  setters: {};
  data: {
    appId: string;
    server: main.ServerInfo;
    system: main.SystemInfo;
    build: main.BuildInfo;
    settings: main.Settings;
    isLoading: boolean;
    aboutSignal: number;
  };
  actions: {
    openAbout: () => void;
  };
};

export const AppContext = createContext({} as AppContextType);

interface AppContextWrapper {
  children: React.ReactNode;
}

export const AppContextWrapper = ({ children }: AppContextWrapper) => {
  const [app, setApp] = useState<main.AppVariable | null>(null);
  const [aboutSignal, setAboutSignal] = useState(0);

  const openAbout = useCallback(() => {
    setAboutSignal((prev) => prev + 1);
  }, []);

  const data = {
    appId: app?.appId || "",
    server: app?.server || ({} as main.ServerInfo),
    system: app?.system || ({} as main.SystemInfo),
    build: app?.build || ({} as main.BuildInfo),
    settings: app?.settings || ({} as main.Settings),
    isLoading: app === null,
    aboutSignal,
  };
  const setters = {};
  const actions = { openAbout };

  useEffect(() => {
    let cancelled = false;
    let retryId: number | null = null;

    const fetchAppContext = async () => {
      try {
        const variables = await AppVariable();
        if (cancelled) return;
        setApp(main.AppVariable.createFrom(variables));
      } catch (error) {
        console.error("Failed to fetch app context:", error);
        if (!cancelled) {
          retryId = window.setTimeout(fetchAppContext, RETRY_INTERVAL);
        }
      }
    };

    const unsubscribe = onEvent<main.AppVariable>("app:variables_changed", (variables) => {
      if (variables) {
        setApp(main.AppVariable.createFrom(variables));
      }
    });

    fetchAppContext();

    return () => {
      cancelled = true;
      if (retryId !== null) clearTimeout(retryId);
      if (unsubscribe) unsubscribe();
    };
  }, []);

  return (
    <AppContext.Provider value={{ data, setters, actions }}>
      {children}
    </AppContext.Provider>
  );
};
