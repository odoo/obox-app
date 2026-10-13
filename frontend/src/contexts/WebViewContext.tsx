import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
} from "react";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { AppContext } from "./AppContext";
import { backendService } from "../services/backend";

const POLL_INTERVAL = 2000;

export type WebViewConfig = {
  url: string;
  enabled: boolean;
  hasPIN: boolean;
  exitCorners?: string[];
  fullscreen?: boolean;
  reloadCount?: number;
};

type WebViewContextType = {
  data: {
    config: WebViewConfig | null;
    isKioskActive: boolean;
    reloadNonce: number;
  };
  actions: {
    saveURL: (url: string) => Promise<void>;
    savePIN: (pin: string) => Promise<void>;
    saveExitCorners: (corners: string[]) => Promise<void>;
    saveFullscreen: (fullscreen: boolean) => Promise<void>;
    toggleEnabled: (v: boolean) => Promise<void>;
    validatePIN: (pin: string) => Promise<boolean>;
    exitKiosk: () => Promise<void>;
    enterKiosk: () => Promise<void>;
    reloadKiosk: () => void;
    refresh: () => Promise<void>;
  };
};

export const WebViewContext = createContext({} as WebViewContextType);

interface WebViewContextWrapperProps {
  children: React.ReactNode;
}

export const WebViewContextWrapper = ({
  children,
}: WebViewContextWrapperProps) => {
  const { data: { isWails } } = useContext(AppContext);
  const [config, setConfig] = useState<WebViewConfig | null>(null);
  const [isKioskActive, setIsKioskActive] = useState(false);
  const [reloadNonce, setReloadNonce] = useState(0);

  const refresh = useCallback(async () => {
    try {
      const cfg: WebViewConfig = await backendService.getWebViewConfig();
      if (backendService.isWails) {
        try {
          const isRendering = await backendService.isRenderingWebApp();
          setIsKioskActive(isRendering);
        } catch {
          /* ignore */
        }
      }

      setConfig((prev) => {
        if (!prev) {
          return cfg;
        }

        if (
          typeof cfg.reloadCount === "number" &&
          typeof prev.reloadCount === "number" &&
          cfg.reloadCount > prev.reloadCount
        ) {
          setReloadNonce((n) => n + 1);
        }

        const prevCorners = (prev.exitCorners || []).join(",");
        const nextCorners = (cfg.exitCorners || []).join(",");
        if (
          prev.url === cfg.url &&
          prev.enabled === cfg.enabled &&
          prev.hasPIN === cfg.hasPIN &&
          prev.fullscreen === cfg.fullscreen &&
          prevCorners === nextCorners &&
          prev.reloadCount === cfg.reloadCount
        ) {
          return prev;
        }

        return cfg;
      });
    } catch (err) {
      console.error("Failed to fetch WebView config:", err);
    }
  }, []);

  // Continuous polling for both browser (Edge on 127.0.0.1 and remote admin) and Wails
  useEffect(() => {
    let timer: number | null = null;
    let mounted = true;

    const poll = async () => {
      await refresh();
      if (mounted) {
        timer = window.setTimeout(poll, POLL_INTERVAL);
      }
    };

    poll();

    return () => {
      mounted = false;
      if (timer) clearTimeout(timer);
    };
  }, [refresh]);

  // Listen for desktop Wails events when config or kiosk state is modified
  useEffect(() => {
    if (!isWails) return;

    const unsubKiosk = EventsOn("kiosk-state-changed", async (enabled: boolean) => {
      setIsKioskActive(enabled);
      try {
        const cfg = await backendService.getWebViewConfig();
        setConfig(cfg);
        await backendService.setWindowFullscreen(enabled ? (cfg.fullscreen ?? true) : false);
      } catch {
        /* ignore */
      }
    });

    const unsubConfig = EventsOn("webview-config-changed", async () => {
      try {
        const cfg = await backendService.getWebViewConfig();
        setConfig(cfg);
      } catch {
        /* ignore */
      }
    });

    const unsubReload = EventsOn("kiosk-reload", () => {
      setReloadNonce((n) => n + 1);
    });

    return () => {
      unsubKiosk();
      unsubConfig();
      unsubReload();
    };
  }, [isWails]);

  // ── Actions ────────────────────────────────────────────────────────────────

  const saveURL = async (url: string) => {
    await backendService.setWebViewURL(url);
    const cfg = await backendService.getWebViewConfig();
    setConfig(cfg);
  };

  const savePIN = async (pin: string) => {
    await backendService.setWebViewPIN(pin);
    const cfg = await backendService.getWebViewConfig();
    setConfig(cfg);
  };

  const saveExitCorners = async (corners: string[]) => {
    await backendService.setWebViewExitCorners(corners);
    const cfg = await backendService.getWebViewConfig();
    setConfig(cfg);
  };

  const saveFullscreen = async (fullscreen: boolean) => {
    await backendService.setWebViewFullscreen(fullscreen);
    const cfg = await backendService.getWebViewConfig();
    setConfig(cfg);
  };

  const toggleEnabled = async (v: boolean) => {
    await backendService.setWebViewEnabled(v);
    const cfg = await backendService.getWebViewConfig();
    setConfig(cfg);
  };

  const enterKiosk = async () => {
    await backendService.setWebViewEnabled(true);
    setIsKioskActive(true);
    if (backendService.isWails) {
      await backendService.setWindowFullscreen(config?.fullscreen ?? true);
      await backendService.navigateToWebApp();
    }
    const cfg = await backendService.getWebViewConfig();
    setConfig(cfg);
  };

  const exitKiosk = async () => {
    await backendService.setWebViewEnabled(false);
    setIsKioskActive(false);
    if (backendService.isWails) {
      await backendService.setWindowFullscreen(false);
      await backendService.completePinAuth(true);
    }
    const cfg = await backendService.getWebViewConfig();
    setConfig(cfg);
  };

  const reloadKiosk = async () => {
    setReloadNonce((n) => n + 1);
    try {
      await backendService.reloadKiosk();
    } catch (err) {
      console.error("Failed to trigger kiosk reload:", err);
    }
  };

  const validatePIN = async (pin: string): Promise<boolean> => {
    return backendService.validatePIN(pin);
  };

  return (
    <WebViewContext.Provider
      value={{
        data: { config, isKioskActive, reloadNonce },
        actions: {
          saveURL,
          savePIN,
          saveExitCorners,
          saveFullscreen,
          toggleEnabled,
          validatePIN,
          enterKiosk,
          exitKiosk,
          reloadKiosk,
          refresh,
        },
      }}
    >
      {children}
    </WebViewContext.Provider>
  );
};
