import { printer } from "../../wailsjs/go/models";
import {
  AddLANPrinter,
  CheckLANPrinterStatus,
  ConfirmRemoveLANPrinter,
  IsNetworkPrintingEnabled,
  Printers,
} from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";

import { createContext, useCallback, useEffect, useRef, useState } from "react";

const POLL_INTERVAL = 5000;
const FETCH_ERROR = "Failed to retrieve printer status. Please try again.";

type PrinterLanStatusByIp = Record<string, "loading" | "online" | "offline">;

type ActionStatus = {
  status: boolean;
  message: string;
};

type PrinterContextType = {
  setters: {};
  data: {
    printers: printer.DiscoveryResult | null;
    lanStatus: PrinterLanStatusByIp;
    fetchError: string | null;
    networkPrintingEnabled: boolean;
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
  const [printers, setPrinters] = useState<printer.DiscoveryResult | null>(null);
  const [lanStatus, setLanStatus] = useState<PrinterLanStatusByIp>({});
  const [fetchError, setFetchError] = useState<string | null>(null);
  const [networkPrintingEnabled, setNetworkPrintingEnabledState] = useState(false);

  // A status sweep can outlast the poll interval (USB rescan plus a 3s dial
  // timeout per unreachable LAN printer), so ticks skip while one is running.
  const statusChecksInFlight = useRef(0);
  const pendingLanChecks = useRef<Set<string>>(new Set());

  const checkLanPrinterStatus = useCallback(async (ip: string) => {
    if (pendingLanChecks.current.has(ip)) {
      return;
    }

    pendingLanChecks.current.add(ip);
    setLanStatus((prevStatus) =>
      prevStatus[ip] === undefined
        ? { ...prevStatus, [ip]: "loading" }
        : prevStatus,
    );

    try {
      const status = await CheckLANPrinterStatus(ip);
      setLanStatus((prevStatus) => ({
        ...prevStatus,
        [ip]: status ? "online" : "offline",
      }));
    } catch (error) {
      console.error(`Failed to check LAN printer status for ${ip}:`, error);
    } finally {
      pendingLanChecks.current.delete(ip);
    }
  }, []);

  // `force` is for refreshes triggered by a user action: those must not be
  // dropped just because a poll happens to be in flight, and the in-flight
  // sweep may have read the printer list before the action landed.
  const checkAppStatus = useCallback(
    async (force = false) => {
      if (statusChecksInFlight.current > 0 && !force) {
        return;
      }

      statusChecksInFlight.current++;
      try {
        const data = await Printers();
        setPrinters(data);
        setFetchError(null);

        for (const printer of data.printers) {
          if (printer.isLAN && printer.lanIp) {
            checkLanPrinterStatus(printer.lanIp);
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

  const removeLanPrinter = async (printer: printer.Device) => {
    if (!printer.isLAN || !printer.lanIp) {
      console.error("Attempted to remove a non-LAN printer:", printer);
      return {
        status: false,
        message: "Cannot remove a non-LAN printer",
      };
    }

    try {
      const confirmed = await ConfirmRemoveLANPrinter(printer.lanIp);
      if (!confirmed) {
        throw new Error("User cancelled the removal of the LAN printer");
      }

      await checkAppStatus(true);
      return {
        status: true,
        message: `Successfully removed LAN printer with IP ${printer.lanIp}`,
      };
    } catch (error) {
      console.error(
        `Failed to remove LAN printer with IP ${printer.lanIp}:`,
        error,
      );
      return {
        status: false,
        message: `Failed to remove LAN printer with IP ${printer.lanIp}: ${error}`,
      };
    }
  };

  const addLanPrinter = async (ip: string) => {
    try {
      await AddLANPrinter(ip);
      await checkAppStatus(true);
      return {
        status: true,
        message: `Successfully added LAN printer with IP ${ip}`,
      };
    } catch (error) {
      console.error(`Failed to add LAN printer with IP ${ip}:`, error);
      return {
        status: false,
        message: `Failed to add LAN printer with IP ${ip}: ${error}`,
      };
    }
  };

  // Only poll while the window is in front: every tick enumerates USB devices
  // and dials each LAN printer, which is wasted work when nobody is looking.
  useEffect(() => {
    let intervalId: number | null = null;

    const startPolling = () => {
      if (intervalId !== null) {
        return;
      }
      checkAppStatus();
      intervalId = window.setInterval(checkAppStatus, POLL_INTERVAL);
    };

    const stopPolling = () => {
      if (intervalId === null) {
        return;
      }
      clearInterval(intervalId);
      intervalId = null;
    };

    const handleVisibilityChange = () =>
      document.hidden ? stopPolling() : startPolling();

    document.addEventListener("visibilitychange", handleVisibilityChange);
    window.addEventListener("focus", startPolling);
    window.addEventListener("blur", stopPolling);

    if (!document.hidden) {
      startPolling();
    }

    return () => {
      stopPolling();
      document.removeEventListener("visibilitychange", handleVisibilityChange);
      window.removeEventListener("focus", startPolling);
      window.removeEventListener("blur", stopPolling);
    };
  }, [checkAppStatus]);

  const loadNetworkPrintingStatus = useCallback(async () => {
    try {
      const enabled = await IsNetworkPrintingEnabled();
      setNetworkPrintingEnabledState(enabled);
    } catch (err) {
      console.error("Failed to load network printing status", err);
    }
  }, []);

  useEffect(() => {
    loadNetworkPrintingStatus();
  }, [loadNetworkPrintingStatus]);

  useEffect(() => {
    return EventsOn("network-printing-changed", () => {
      loadNetworkPrintingStatus();
      checkAppStatus(true);
    });
  }, [loadNetworkPrintingStatus, checkAppStatus]);

  const setters = {};
  const actions = {
    removeLanPrinter,
    addLanPrinter,
  };
  const data = {
    printers: printers,
    lanStatus,
    fetchError,
    networkPrintingEnabled,
  };

  return (
    <PrinterContext.Provider value={{ data, setters, actions }}>
      {children}
    </PrinterContext.Provider>
  );
};
