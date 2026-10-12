import { createContext, useCallback, useContext, useEffect, useRef, useState } from "react";
import { printer } from "../../wailsjs/go/models";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { AppContext } from "./AppContext";
import { backendService } from "../services/backend";
import { usePINGate } from "../hooks/usePINGate";

const POLL_INTERVAL = 5000;
const FETCH_ERROR = "Failed to retrieve printer status. Please try again.";

type PrinterLanStatusByIp = Record<string, "loading" | "online" | "offline">;

type ActionStatus = {
  status: boolean;
  message: string;
};

type PrinterContextType = {
  setters: Record<string, never>;
  data: {
    printers: printer.DiscoveryResult | null;
    lanStatus: PrinterLanStatusByIp;
    fetchError: string | null;
  };
  actions: {
    removeLanPrinter: (printer: printer.Device) => Promise<ActionStatus>;
    addLanPrinter: (ip: string) => Promise<ActionStatus>;
  };
};

export const PrinterContext = createContext({} as PrinterContextType);

interface PrinterContextWrapper {
  children: React.ReactNode;
}

export const PrinterContextWrapper = ({ children }: PrinterContextWrapper) => {
  const { data: { isWails } } = useContext(AppContext);
  const gate = usePINGate();
  const [printers, setPrinters] = useState<printer.DiscoveryResult | null>(null);
  const [lanStatus, setLanStatus] = useState<PrinterLanStatusByIp>({});
  const [fetchError, setFetchError] = useState<string | null>(null);

  const statusChecksInFlight = useRef(0);
  const pendingLanChecks = useRef<Set<string>>(new Set());

  const checkLanPrinterStatus = useCallback(async (ip: string) => {
    if (pendingLanChecks.current.has(ip)) return;
    pendingLanChecks.current.add(ip);
    setLanStatus((prev) =>
      prev[ip] === undefined ? { ...prev, [ip]: "loading" } : prev,
    );
    try {
      const result = await backendService.checkLANPrinterStatus(ip);
      setLanStatus((prev) => ({ ...prev, [ip]: result.online ? "online" : "offline" }));
    } catch (error) {
      console.error(`Failed to check LAN printer status for ${ip}:`, error);
    } finally {
      pendingLanChecks.current.delete(ip);
    }
  }, []);

  const checkAppStatus = useCallback(
    async (force = false) => {
      if (statusChecksInFlight.current > 0 && !force) return;
      statusChecksInFlight.current++;
      try {
        const data = (await backendService.getPrinters()) as unknown as printer.DiscoveryResult;
        setPrinters(data);
        setFetchError(null);
        for (const p of data.printers) {
          if (p.isLAN && p.lanIp) {
            checkLanPrinterStatus(p.lanIp);
          }
        }
      } catch (error) {
        console.error("Failed to check app status:", error);
        setFetchError(FETCH_ERROR);
      } finally {
        statusChecksInFlight.current--;
      }
    },
    [checkLanPrinterStatus],
  );

  const removeLanPrinter = async (
    printer: printer.Device,
  ): Promise<ActionStatus> => {
    const lanIp = printer.lanIp;
    if (!printer.isLAN || !lanIp) {
      return { status: false, message: "Cannot remove a non-LAN printer" };
    }
    const res = await gate(async () => {
      try {
        if (!isWails) {
          const confirmed = window.confirm(
            `Are you sure you want to remove the printer at ${lanIp}?`,
          );
          if (!confirmed) {
            return { status: false, message: "User cancelled the removal" };
          }
        }
        const confirmed = await backendService.removeLANPrinter(lanIp);
        if (!confirmed) throw new Error("User cancelled the removal");
        await checkAppStatus(true);
        return {
          status: true,
          message: `Successfully removed LAN printer with IP ${lanIp}`,
        };
      } catch (error) {
        return {
          status: false,
          message: `Failed to remove LAN printer: ${error}`,
        };
      }
    });

    if (res === null) {
      return { status: false, message: "Action cancelled or PIN required" };
    }
    return res;
  };

  const addLanPrinter = async (ip: string): Promise<ActionStatus> => {
    const res = await gate(async () => {
      try {
        await backendService.addLANPrinter(ip);
        await checkAppStatus(true);
        return { status: true, message: `Successfully added LAN printer with IP ${ip}` };
      } catch (error) {
        return {
          status: false,
          message: `Failed to add LAN printer with IP ${ip}: ${error}`,
        };
      }
    });

    if (res === null) {
      return { status: false, message: "Action cancelled or PIN required" };
    }
    return res;
  };

  // Poll while window is visible
  useEffect(() => {
    let intervalId: number | null = null;

    const startPolling = () => {
      if (intervalId !== null) return;
      checkAppStatus();
      intervalId = window.setInterval(checkAppStatus, POLL_INTERVAL);
    };

    const stopPolling = () => {
      if (intervalId === null) return;
      clearInterval(intervalId);
      intervalId = null;
    };

    const handleVisibilityChange = () =>
      document.hidden ? stopPolling() : startPolling();

    document.addEventListener("visibilitychange", handleVisibilityChange);
    window.addEventListener("focus", startPolling);
    window.addEventListener("blur", stopPolling);

    if (!document.hidden) startPolling();

    return () => {
      stopPolling();
      document.removeEventListener("visibilitychange", handleVisibilityChange);
      window.removeEventListener("focus", startPolling);
      window.removeEventListener("blur", stopPolling);
    };
  }, [checkAppStatus]);

  // Refresh printers if app variables change (e.g. network printing toggled)
  useEffect(() => {
    if (!isWails) return;
    return EventsOn("app:variables_changed", () => {
      checkAppStatus(true);
    });
  }, [isWails, checkAppStatus]);

  const setters = {} as Record<string, never>;
  const actions = { removeLanPrinter, addLanPrinter };
  const data = { printers, lanStatus, fetchError };

  return (
    <PrinterContext.Provider value={{ data, setters, actions }}>
      {children}
    </PrinterContext.Provider>
  );
};
