import { useContext } from "react";
import NetworkIpDialog from "./NetworkIpDialog";
import PrinterListItem from "./PrinterListItem";
import OdooStatus from "./OdooStatus";
import { PrinterContext } from "../contexts/PrinterContext";
import { AppContext } from "../contexts/AppContext";
import { ToastContext } from "../contexts/ToastContext";
import { PINContext } from "../contexts/PINContext";
import { backendService } from "../services/backend";
import { DisconnectIcon } from "../functions/icon";
// import TroubleshootDialog from "./TroubleshootDialog";

export default function PrinterList() {
  const printerContext = useContext(PrinterContext);
  const { data: { isWindows, isKioskMode } } = useContext(AppContext);
  const toastContext = useContext(ToastContext);
  const { showPINDialog } = useContext(PINContext);

  const { printers, fetchError } = printerContext.data;
  const errorMessage = fetchError ?? printers?.errorMsg;

  const isLocalhost =
    typeof window !== "undefined" &&
    (window.location.hostname === "127.0.0.1" ||
      window.location.hostname === "localhost");

  const isWindowsKioskServer =
    Boolean(isWindows) &&
    Boolean(isKioskMode) &&
    isLocalhost;

  const handleQuitServer = async () => {
    const ok = await showPINDialog();
    if (!ok) return;

    try {
      await backendService.quitServer();
      toastContext.actions.showToast("ePOS Proxy server stopped", "success");
    } catch (err: unknown) {
      toastContext.actions.showToast("Failed to stop server: " + String(err), "danger");
    }
  };

  return (
    <div className="w-full flex flex-col min-h-0">
      <div className="w-full bg-white/85 rounded-2xl shadow-lg overflow-hidden flex flex-col min-h-0">
        {printers && (printers.printers.length > 0 || printers.unavailablePrinters.length > 0) && (
          <div className="p-4 sm:p-6 overflow-y-auto max-h-[calc(100vh-280px)] sm:max-h-[calc(100vh-260px)]">
            <ul className="divide-y divide-gray-300">
              {printers.printers.map((printer) => (
                <PrinterListItem
                  key={printer.id}
                  printer={printer}
                  isOnline={true}
                />
              ))}
              {printers.unavailablePrinters.map((printer) => (
                <PrinterListItem
                  key={printer.name}
                  printer={printer}
                  isOnline={false}
                />
              ))}
            </ul>
          </div>
        )}

        {!printers ? (
          !fetchError && (
            <div className="p-4 sm:p-6 animate-pulse space-y-3">
              <div className="flex items-center gap-2.5">
                <div className="w-3 h-3 rounded-full bg-gray-200" />
                <div className="h-4 bg-gray-200 rounded w-44" />
              </div>
              <div className="h-3 bg-gray-100 rounded w-28 ml-5.5" />
              <div className="flex gap-2 pt-1">
                <div className="h-8 bg-gray-100 rounded-lg flex-1" />
                <div className="h-8 bg-gray-100 rounded-lg flex-1" />
              </div>
            </div>
          )
        ) : (
          printers.printers.length === 0 &&
          printers.unavailablePrinters.length === 0 && (
            <div className="p-6">
              <div className="font-medium text-lg text-center">
                No printers found
              </div>
              <div className="mt-2 text-gray-600 text-center">
                Make sure your printer is powered on and connected via USB.
              </div>
            </div>
          )
        )}

        {errorMessage && (
          <div className="px-6 pb-4">
            <div className="text-red-700 text-center">
              Error: {errorMessage}
            </div>
          </div>
        )}
      </div>

      <div className="mt-6 w-full grid grid-cols-2 gap-2.5 shrink-0 h-12">
        <OdooStatus />
        <NetworkIpDialog />
        {/* <TroubleshootDialog /> */}
        {isWindowsKioskServer && (
          <>
            <button
              type="button"
              onClick={handleQuitServer}
              className="w-full flex items-center justify-between rounded-xl border border-red-200 bg-white px-4 py-3 text-red-600 hover:bg-red-50 transition-colors cursor-pointer shadow-2xs"
            >
              <div className="flex items-center gap-3">
                <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-red-50 text-red-600">
                  <DisconnectIcon className="h-5 w-5" />
                </div>
                <div className="text-left">
                  <div className="text-sm font-semibold text-gray-800">
                    Stop ePOS Proxy
                  </div>
                  <div className="text-xs text-gray-500">
                    Shut down background server process
                  </div>
                </div>
              </div>
              <span className="rounded-full bg-red-50 border border-red-200 px-2.5 py-1 text-[10px] font-semibold uppercase tracking-wide text-red-600">
                Quit
              </span>
            </button>
          </>
        )}
      </div>
    </div>
  );
}
