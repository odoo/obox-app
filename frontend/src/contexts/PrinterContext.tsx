import { main } from "../../wailsjs/go/models";
import {
  AddPrinter,
  CheckLANPrinterStatus,
  ConfirmRemoveLANPrinter,
  CheckBluetoothPrinterStatus,
  ConfirmRemoveBluetoothPrinter,
  IsNetworkPrintingEnabled,
  Printers,
} from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { createContext, useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";

const POLL_INTERVAL = 5000;
const FETCH_ERROR = "Failed to retrieve printer status. Please try again.";

export type PrinterStatus = "loading" | "online" | "offline";
export type PrinterStatusMap = Record<string, PrinterStatus>;

export type ActionStatus = {
  status: boolean;
  message: string;
};

export type AddPrinterParams = {
  connectionType: "lan" | "bluetooth";
  address: string;
  name?: string;
  protocol?: string;
  bottomPadding?: number;
};

export type UIPrinter = main.Printer & {
  status: PrinterStatus;
  remove?: () => Promise<ActionStatus>;
};

export type PrinterContextType = {
  setters: {};
  data: {
    printers: UIPrinter[];
    unavailablePrinters: main.UnavailablePrinter[];
    isLoading: boolean;
    errorMsg: string | null;
    networkPrintingEnabled: boolean;
  };
  actions: {
    addPrinter: (params: AddPrinterParams) => Promise<ActionStatus>;
    checkPrinterStatus: (printer: main.Printer) => Promise<void>;
    refreshPrinters: (force?: boolean) => Promise<void>;
  };
};

export const PrinterContext = createContext({} as PrinterContextType);

export const PrinterContextWrapper = ({ children }: { children: ReactNode }) => {
  const [rawPrinters, setRawPrinters] = useState<main.Printers | null>(null);
  const [status, setStatus] = useState<PrinterStatusMap>({});
  const [fetchError, setFetchError] = useState<string | null>(null);
  const [networkPrintingEnabled, setNetworkPrintingEnabledState] = useState(false);

  const statusChecksInFlight = useRef(0);
  const pendingChecks = useRef<Set<string>>(new Set());

  const checkPrinterStatus = useCallback(async (printer: main.Printer) => {
    const isLan = printer.connectionType === "lan" && !!printer.lanIp;
    const isBt = printer.connectionType === "bluetooth" && !!printer.btMac;
    if (!isLan && !isBt) return;

    const key = printer.id;
    if (pendingChecks.current.has(key)) return;
    pendingChecks.current.add(key);

    setStatus((prev) => (prev[key] === undefined ? { ...prev, [key]: "loading" } : prev));

    try {
      const isOnline = isLan
        ? await CheckLANPrinterStatus(printer.lanIp!)
        : await CheckBluetoothPrinterStatus(printer.btMac!);
      setStatus((prev) => ({ ...prev, [key]: isOnline ? "online" : "offline" }));
    } catch (error) {
      console.error(`Failed to check printer status for ${printer.name || key}:`, error);
      setStatus((prev) => ({ ...prev, [key]: "offline" }));
    } finally {
      pendingChecks.current.delete(key);
    }
  }, []);

  const checkAppStatus = useCallback(
    async (force = false) => {
      if (statusChecksInFlight.current > 0 && !force) return;

      statusChecksInFlight.current++;
      try {
        const data = await Printers();
        setRawPrinters(data);
        setFetchError(null);

        for (const printer of data.printers) {
          checkPrinterStatus(printer);
        }
      } catch (error) {
        console.error("Failed to check app status:", error);
        setFetchError(FETCH_ERROR);
      } finally {
        statusChecksInFlight.current--;
      }
    },
    [checkPrinterStatus],
  );

  const removePrinter = useCallback(
    async (printer: main.Printer): Promise<ActionStatus> => {
      try {
        if (printer.connectionType === "lan" && printer.lanIp) {
          const confirmed = await ConfirmRemoveLANPrinter(printer.lanIp);
          if (!confirmed) throw new Error("User cancelled removal");
          await checkAppStatus(true);
          return { status: true, message: `Successfully removed LAN printer with IP ${printer.lanIp}` };
        }
        if (printer.connectionType === "bluetooth" && printer.btMac) {
          const confirmed = await ConfirmRemoveBluetoothPrinter(printer.btMac);
          if (!confirmed) throw new Error("User cancelled removal");
          await checkAppStatus(true);
          return { status: true, message: `Successfully removed Bluetooth printer ${printer.name || printer.btMac}` };
        }
        return { status: false, message: "Cannot remove this printer" };
      } catch (error) {
        return { status: false, message: `Failed to remove printer: ${error}` };
      }
    },
    [checkAppStatus],
  );

  const addPrinter = async (params: AddPrinterParams): Promise<ActionStatus> => {
    try {
      await AddPrinter({
        connectionType: params.connectionType,
        address: params.address,
        name: params.name ?? params.address,
        protocol: params.protocol ?? "ESCPOS",
        bottomPadding: params.bottomPadding ?? 0,
      });
      await checkAppStatus(true);
      return { status: true, message: `Successfully added printer ${params.name || params.address}` };
    } catch (error) {
      return { status: false, message: `Failed to add printer: ${error}` };
    }
  };

  const enrichedPrinters: UIPrinter[] = useMemo(() => {
    if (!rawPrinters?.printers) return [];

    return rawPrinters.printers.map((p) => {
      const isRemovable = p.connectionType === "lan" || p.connectionType === "bluetooth";
      const currentStatus: PrinterStatus =
        p.connectionType === "usb"
          ? (p.online ? "online" : "offline")
          : (status[p.id] ?? "loading");

      return {
        ...p,
        status: currentStatus,
        remove: isRemovable ? () => removePrinter(p) : undefined,
      };
    });
  }, [rawPrinters, status, removePrinter]);

  // Poll only while window is visible and focused
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

    const handleVisibilityChange = () => (document.hidden ? stopPolling() : startPolling());

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

  return (
    <PrinterContext.Provider
      value={{
        data: {
          printers: enrichedPrinters,
          unavailablePrinters: rawPrinters?.unavailablePrinters ?? [],
          isLoading: !rawPrinters && !fetchError,
          errorMsg: fetchError ?? rawPrinters?.errorMsg ?? null,
          networkPrintingEnabled,
        },
        setters: {},
        actions: {
          addPrinter,
          checkPrinterStatus,
          refreshPrinters: checkAppStatus,
        },
      }}
    >
      {children}
    </PrinterContext.Provider>
  );
};
