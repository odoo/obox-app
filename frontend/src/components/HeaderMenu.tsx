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
  SettingsIcon,
  WifiIcon,
} from "../functions/icon";
import { AppContext } from "../contexts/AppContext";

interface ToggleSwitchProps {
  checked: boolean;
  disabled?: boolean;
  isLoading?: boolean;
  onChange: () => void;
}

function ToggleSwitch({ checked, disabled, isLoading, onChange }: ToggleSwitchProps) {
  if (isLoading) {
    return <div className="h-5 w-9 shrink-0 rounded-full bg-gray-200 animate-pulse" />;
  }

  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      onClick={(e) => {
        e.stopPropagation();
        onChange();
      }}
      className={`inline-flex h-5 w-9 shrink-0 p-0.5 rounded-full transition-colors cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed ${checked ? "bg-odoo" : "bg-gray-300"
        }`}
    >
      <span
        aria-hidden="true"
        className={`h-4 w-4 rounded-full bg-white shadow-sm transition-transform ${checked ? "translate-x-4" : "translate-x-0"
          }`}
      />
    </button>
  );
}

interface ConfigToggleItemProps {
  icon: React.ReactNode;
  iconBg: string;
  title: string;
  description: string;
  checked: boolean;
  disabled: boolean;
  isLoading: boolean;
  onToggle: () => void;
}

function ConfigToggleItem({
  icon,
  iconBg,
  title,
  description,
  checked,
  disabled,
  isLoading,
  onToggle,
}: ConfigToggleItemProps) {
  const isInteractive = !disabled && !isLoading;
  return (
    <div
      onClick={isInteractive ? onToggle : undefined}
      className={`flex items-center justify-between gap-3 px-3 py-2.5 rounded-xl transition-colors ${
        isInteractive ? "hover:bg-gray-50 cursor-pointer" : "cursor-default"
      }`}
    >
      <div className="flex items-center gap-2.5 min-w-0">
        <div className={`w-7 h-7 rounded-lg flex items-center justify-center shrink-0 border ${iconBg}`}>
          {icon}
        </div>
        <div className="min-w-0">
          <div className="text-xs font-medium text-gray-800">{title}</div>
          <div className="text-[10px] text-gray-500 truncate">{description}</div>
        </div>
      </div>
      <ToggleSwitch
        checked={checked}
        disabled={disabled || isLoading}
        isLoading={isLoading}
        onChange={onToggle}
      />
    </div>
  );
}

interface ConfigActionItemProps {
  icon: React.ReactNode;
  iconBg: string;
  title: string;
  description: string;
  onClick: () => void;
}

function ConfigActionItem({ icon, iconBg, title, description, onClick }: ConfigActionItemProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="w-full flex items-center gap-2.5 px-3 py-2 rounded-xl text-xs font-medium text-gray-700 hover:bg-gray-50 hover:text-odoo cursor-pointer transition-colors text-left"
    >
      <div className={`w-7 h-7 rounded-lg flex items-center justify-center shrink-0 border ${iconBg}`}>
        {icon}
      </div>
      <div className="min-w-0">
        <div className="text-xs font-medium text-gray-800">{title}</div>
        <div className="text-[10px] text-gray-500 truncate">{description}</div>
      </div>
    </button>
  );
}

export default function HeaderMenu() {
  const menuRef = useRef<HTMLDivElement>(null);
  const appContext = useContext(AppContext);
  const toastContext = useContext(ToastContext);

  const [isOpen, setIsOpen] = useState(false);
  const [isLoadingAutostart, setIsLoadingAutostart] = useState(false);
  const [isLoadingNetwork, setIsLoadingNetwork] = useState(false);
  const { autoStart, networkPrintingEnabled, isLoading } = appContext.data;

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

  const toggleSetting = async (
    action: () => Promise<unknown>,
    setLoading: (loading: boolean) => void,
    name: string,
    currentValue: boolean
  ) => {
    if (isLoading) return;
    setLoading(true);
    try {
      await action();
      toastContext.actions.showToast(`${name} ${currentValue ? "Disabled" : "Enabled"}`, "success");
    } catch (err) {
      toastContext.actions.showToast(`Failed to update ${name.toLowerCase()}: ${errorText(err, "unknown error")}`, "danger");
    } finally {
      setLoading(false);
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
            <ConfigToggleItem
              icon={<BoltIcon className="w-3.5 h-3.5" />}
              iconBg="bg-purple-50 border-purple-100/80 text-odoo"
              title="Auto start"
              description="Launch on system login"
              checked={autoStart}
              disabled={isLoadingAutostart}
              isLoading={isLoading}
              onToggle={() => toggleSetting(ToggleAutoStart, setIsLoadingAutostart, "Auto start", autoStart)}
            />
            <ConfigToggleItem
              icon={<WifiIcon className="w-3.5 h-3.5" />}
              iconBg="bg-blue-50 border-blue-100/80 text-blue-600"
              title="Accept print requests"
              description="Accept LAN print jobs from other devices"
              checked={networkPrintingEnabled}
              disabled={isLoadingNetwork}
              isLoading={isLoading}
              onToggle={() => toggleSetting(ToggleNetworkPrinting, setIsLoadingNetwork, "Network printing", networkPrintingEnabled)}
            />
          </div>

          <div className="border-t border-gray-100 my-1.5" />

          <div className="space-y-1">
            <ConfigActionItem
              icon={<DownloadIcon className="w-3.5 h-3.5" />}
              iconBg="bg-amber-50 border-amber-100/80 text-amber-600"
              title="Download Logs"
              description="Export logs archive for debug"
              onClick={handleDownloadLogs}
            />
          </div>
        </div>
      )}
    </div>
  );
}
