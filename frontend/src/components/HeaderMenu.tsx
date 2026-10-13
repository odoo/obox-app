import { useContext, useEffect, useRef, useState } from "react";
import {
  DownloadLogs,
  ToggleAutoStart,
  ToggleNetworkPrinting,
} from "../../wailsjs/go/main/App";
import { ToastContext } from "../contexts/ToastContext";
import { errorText } from "../error";
import {
  BoltIcon,
  DownloadIcon,
  GlobeAltIcon,
  MonitorIcon,
  SettingsIcon,
  SpinnerIcon,
  WifiIcon,
} from "../functions/icon";
import { AppContext } from "../contexts/AppContext";
import { WebViewContext } from "../contexts/WebViewContext";
import WebViewDialog from "./WebViewDialog";
import OdooSessionsDialog from "./OdooSessionsDialog";

export default function HeaderMenu() {
  const menuRef = useRef<HTMLDivElement>(null);
  const appContext = useContext(AppContext);
  const toastContext = useContext(ToastContext);
  const { data: webViewData } = useContext(WebViewContext);

  const [isOpen, setIsOpen] = useState(false);
  const [isLoadingAutostart, setIsLoadingAutostart] = useState(false);
  const [isLoadingNetwork, setIsLoadingNetwork] = useState(false);
  const [kioskOpenSignal, setKioskOpenSignal] = useState(0);
  const [odooSessionsOpenSignal, setOdooSessionsOpenSignal] = useState(0);

  const { autoStart, networkPrintingEnabled, isWails } = appContext.data;

  const isLocalhost =
    typeof window !== "undefined" &&
    (window.location.hostname === "127.0.0.1" ||
      window.location.hostname === "localhost");

  const isKioskCurrentlyActive = (isWails || isLocalhost)
    ? webViewData.isKioskActive
    : Boolean(webViewData.config?.enabled);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };

    if (isOpen) {
      document.addEventListener("mousedown", handleClickOutside);
    }
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
    };
  }, [isOpen]);

  const toggleAutostart = async () => {
    setIsLoadingAutostart(true);
    try {
      await ToggleAutoStart();
      toastContext.actions.showToast(`Auto start ${autoStart ? "Disabled" : "Enabled"}`, "success");
    } catch (err) {
      toastContext.actions.showToast(`Failed to update auto start: ${errorText(err, "unknown error")}`, "danger");
    } finally {
      setIsLoadingAutostart(false);
    }
  };

  const toggleNetworkPrinting = async () => {
    setIsLoadingNetwork(true);
    try {
      await ToggleNetworkPrinting();
      toastContext.actions.showToast(`Network printing ${networkPrintingEnabled ? "Disabled" : "Enabled"}`, "success");
    } catch (err) {
      toastContext.actions.showToast(`Failed to update network printing: ${errorText(err, "unknown error")}`, "danger");
    } finally {
      setIsLoadingNetwork(false);
    }
  };

  const handleDownloadLogs = async () => {
    setIsOpen(false);
    try {
      await DownloadLogs();
    } catch (err) {
      toastContext.actions.showToast(`Failed to download logs: ${errorText(err, "unknown error")}`, "danger");
    }
  };

  const handleOpenKiosk = () => {
    setIsOpen(false);
    setKioskOpenSignal((prev) => prev + 1);
  };

  const handleOpenOdooSessions = () => {
    setIsOpen(false);
    setOdooSessionsOpenSignal((prev) => prev + 1);
  };

  return (
    <div className="relative" ref={menuRef}>
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        title="Configurations"
        aria-expanded={isOpen}
        className={`p-2 rounded-xl text-gray-500 hover:text-odoo hover:bg-purple-50 active:scale-95 transition-all cursor-pointer flex items-center justify-center border ${isOpen
          ? "bg-purple-50 text-odoo border-purple-200"
          : "border-transparent hover:border-purple-100"
          }`}
      >
        <SettingsIcon className="w-5 h-5" />
      </button>

      {isOpen && (
        <div className="absolute right-0 top-full mt-2 w-72 max-h-[calc(100vh-80px)] overflow-y-auto bg-white text-gray-800 rounded-2xl shadow-2xl border border-gray-100 p-2 z-50">
          <div className="px-3 pt-2 pb-1.5 text-[11px] font-semibold text-gray-400 uppercase tracking-wider">
            Configurations
          </div>

          <div className="space-y-1">
            {/* Auto Start Item */}
            <button
              type="button"
              disabled={isLoadingAutostart}
              onClick={toggleAutostart}
              className="w-full flex items-center justify-between gap-3 px-3 py-2.5 rounded-xl hover:bg-gray-50 cursor-pointer disabled:opacity-60 transition-colors text-left"
            >
              <div className="flex items-center gap-2.5 min-w-0">
                <div
                  className={`w-7 h-7 rounded-lg flex items-center justify-center shrink-0 transition-colors ${
                    autoStart
                      ? "bg-odoo text-white shadow-xs"
                      : "bg-purple-50 border border-purple-100/80 text-odoo"
                  }`}
                >
                  {isLoadingAutostart ? (
                    <SpinnerIcon className="w-3.5 h-3.5" />
                  ) : (
                    <BoltIcon className="w-3.5 h-3.5" />
                  )}
                </div>
                <div className="min-w-0">
                  <div className="text-xs font-medium text-gray-800">
                    Auto start
                  </div>
                  <div className="text-[10px] text-gray-500 truncate">
                    Launch on system login
                  </div>
                </div>
              </div>
            </button>

            {/* Network Printing Item */}
            <button
              type="button"
              disabled={isLoadingNetwork}
              onClick={toggleNetworkPrinting}
              className="w-full flex items-center justify-between gap-3 px-3 py-2.5 rounded-xl hover:bg-gray-50 cursor-pointer disabled:opacity-60 transition-colors text-left"
            >
              <div className="flex items-center gap-2.5 min-w-0">
                <div
                  className={`w-7 h-7 rounded-lg flex items-center justify-center shrink-0 transition-colors ${
                    networkPrintingEnabled
                      ? "bg-blue-600 text-white shadow-xs"
                      : "bg-blue-50 border border-blue-100/80 text-blue-600"
                  }`}
                >
                  {isLoadingNetwork ? (
                    <SpinnerIcon className="w-3.5 h-3.5" />
                  ) : (
                    <WifiIcon className="w-3.5 h-3.5" />
                  )}
                </div>
                <div className="min-w-0">
                  <div className="text-xs font-medium text-gray-800">
                    Accept print requests
                  </div>
                  <div className="text-[10px] text-gray-500 truncate">
                    Accept LAN print jobs from other devices
                  </div>
                </div>
              </div>
            </button>
          </div>

          <div className="border-t border-gray-100 my-1.5" />

          <div className="space-y-1">
            <button
              type="button"
              onClick={handleOpenKiosk}
              className="w-full flex items-center justify-between gap-2.5 px-3 py-2 rounded-xl text-xs font-medium text-gray-700 hover:bg-gray-50 hover:text-odoo cursor-pointer transition-colors text-left"
            >
              <div className="flex items-center gap-2.5 min-w-0">
                <div
                  className={`w-7 h-7 rounded-lg flex items-center justify-center shrink-0 transition-colors ${
                    isKioskCurrentlyActive
                      ? "bg-emerald-600 text-white shadow-xs"
                      : "bg-emerald-50 border border-emerald-100/80 text-emerald-600"
                  }`}
                >
                  <MonitorIcon className="w-3.5 h-3.5" />
                </div>
                <div className="min-w-0">
                  <div className="text-xs font-medium text-gray-800">
                    Kiosk & Remote Access
                  </div>
                  <div className="text-[10px] text-gray-500 truncate">
                    Configure kiosk, screen & pairing
                  </div>
                </div>
              </div>
            </button>

            <button
              type="button"
              onClick={handleOpenOdooSessions}
              className="w-full flex items-center justify-between gap-2.5 px-3 py-2 rounded-xl text-xs font-medium text-gray-700 hover:bg-gray-50 hover:text-odoo cursor-pointer transition-colors text-left"
            >
              <div className="flex items-center gap-2.5 min-w-0">
                <div className="w-7 h-7 rounded-lg bg-purple-50 border border-purple-100/80 text-odoo flex items-center justify-center shrink-0">
                  <GlobeAltIcon className="w-3.5 h-3.5" />
                </div>
                <div className="min-w-0">
                  <div className="text-xs font-medium text-gray-800">
                    Odoo Sessions
                  </div>
                  <div className="text-[10px] text-gray-500 truncate">
                    Connect & pair with Odoo instances
                  </div>
                </div>
              </div>
            </button>

            <button
              type="button"
              onClick={handleDownloadLogs}
              className="w-full flex items-center gap-2.5 px-3 py-2 rounded-xl text-xs font-medium text-gray-700 hover:bg-gray-50 hover:text-odoo cursor-pointer transition-colors text-left"
            >
              <div className="w-7 h-7 rounded-lg bg-amber-50 border border-amber-100/80 text-amber-600 flex items-center justify-center shrink-0">
                <DownloadIcon className="w-3.5 h-3.5" />
              </div>
              <div className="min-w-0">
                <div className="text-xs font-medium text-gray-800">
                  Download Logs
                </div>
                <div className="text-[10px] text-gray-500 truncate">
                  Export logs archive for debug
                </div>
              </div>
            </button>
          </div>
        </div>
      )}
      <WebViewDialog openSignal={kioskOpenSignal} />
      <OdooSessionsDialog openSignal={odooSessionsOpenSignal} />
    </div>
  );
}
