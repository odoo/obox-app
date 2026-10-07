import { useContext, useEffect, useState } from "react";
import { AppContext } from "../contexts/AppContext";
import { OdooContext } from "../contexts/OdooContext";
import { ToastContext } from "../contexts/ToastContext";
import { errorText } from "../error";
import {
  AppIdIcon,
  CheckIcon,
  CloudStatusIcon,
  CopyIcon,
  DisconnectIcon,
  HelpCircleIcon,
  IpIcon,
  LanStatusIcon,
  LinkIcon,
} from "../functions/icon";
import { useClipboard } from "../hooks/useClipboard";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import Dialog from "./Dialog";

function DisconnectButton({ onClick }: { onClick: (e?: React.MouseEvent) => void; }) {
  return (
    <button
      type="button"
      onClick={onClick}
      title="Disconnect from Odoo"
      aria-label="Disconnect from Odoo"
      className={"p-2 rounded-full text-rose-600 hover:text-rose-600 hover:bg-rose-200/80 transition-colors cursor-pointer shrink-0"}
    >
      <DisconnectIcon className="w-4 h-4" />
    </button>
  );
}

interface CopyButtonProps {
  label: string;
  isCopied: boolean;
  onCopy: () => void;
}

function CopyButton({ label, isCopied, onCopy }: CopyButtonProps) {
  return (
    <button
      type="button"
      onClick={(e) => {
        e.stopPropagation();
        onCopy();
      }}
      title={isCopied ? "Copied!" : `Copy ${label}`}
      aria-label={`Copy ${label}`}
      className={`shrink-0 p-1.5 rounded-lg transition-all duration-150 cursor-pointer flex items-center justify-center ${isCopied
        ? "text-emerald-600 bg-emerald-100/80 ring-1 ring-emerald-300"
        : "text-gray-400 hover:text-odoo hover:bg-purple-50 active:scale-95"
        }`}
    >
      {isCopied ? (
        <CheckIcon className="w-3.5 h-3.5" />
      ) : (
        <CopyIcon className="w-3.5 h-3.5" />
      )}
    </button>
  );
}

interface FieldCardProps {
  label: string;
  value: string;
  title: string;
  iconBg: string;
  icon: React.ReactNode;
  isCopied: boolean;
  onCopy: () => void;
}

function FieldCard({
  label,
  value,
  title,
  iconBg,
  icon,
  isCopied,
  onCopy,
}: FieldCardProps) {
  return (
    <div
      className="flex items-center justify-between gap-2 bg-gray-50 hover:bg-gray-100/70 border border-gray-200/80 rounded-xl px-3 py-2.5 transition-all shadow-2xs"
      title={title}
    >
      <div className="flex items-center gap-2.5 min-w-0">
        <div className={`w-7 h-7 rounded-lg ${iconBg} flex items-center justify-center shrink-0 border`}>
          {icon}
        </div>
        <div className="min-w-0">
          <div className="text-[10px] font-semibold text-gray-400 uppercase tracking-wider">
            {label}
          </div>
          <div className="font-mono text-xs text-gray-800 font-medium truncate">
            {value || "—"}
          </div>
        </div>
      </div>

      <CopyButton label={label} isCopied={isCopied} onCopy={onCopy} />
    </div>
  );
}

interface StatusPillProps {
  type: "lan" | "cloud";
  status: string;
}

function StatusPill({ type, status }: StatusPillProps) {
  const isConnected = status === "connected";
  const isPending =
    type === "cloud"
      ? status === "connecting" || status === "polling"
      : status === "connecting";

  const colorClass = {
    emerald: "text-emerald-600 border-emerald-700 hover:text-emerald-600 hover:bg-emerald-200/80",
    cyan: "text-cyan-600 border-cyan-700 hover:text-cyan-600 hover:bg-cyan-200/80",
    rose: "text-rose-600 border-rose-700 hover:text-rose-600 hover:bg-rose-200/80",
  }[isConnected ? "emerald" : isPending ? "cyan" : "rose"];

  const labelText = type === "lan" ? `Local Network: ${status}` : `Cloud Status: ${status}`;

  const icon = type === "lan" ? (
    <LanStatusIcon className={`w-4 h-4 ${isPending ? "animate-pulse" : ""}`} />
  ) : (
    <CloudStatusIcon className={`w-4 h-4 ${isPending ? "animate-pulse" : ""}`} />
  );

  return (
    <div
      className={`inline-flex items-center p-1.5 rounded-full transition-colors cursor-pointer shrink-0 ${colorClass}`}
      title={labelText}
    >
      {icon}
    </div>
  );
}

function StatusPills({ lanStatus, wsStatus }: { lanStatus: string; wsStatus: string }) {
  return (
    <div className="flex items-center gap-1.5 shrink-0">
      <StatusPill type="lan" status={lanStatus} />
      <StatusPill type="cloud" status={wsStatus} />
    </div>
  );
}

export default function OdooStatus() {
  const appContext = useContext(AppContext);
  const odooContext = useContext(OdooContext);
  const toastContext = useContext(ToastContext);
  const { copy, isCopied } = useClipboard();

  const { data: odooData } = odooContext;
  const { data: appData } = appContext;

  const isConnected = Boolean(odooData.status?.dbUrl);
  const lanStatus = odooData.status?.lanStatus || "disconnected";
  const wsStatus = odooData.status?.websocketStatus || (isConnected ? "connected" : "disconnected");

  const handleDisconnect = async (e?: React.MouseEvent) => {
    e?.stopPropagation();
    try {
      const removed = await odooContext.actions.disconnectOdoo();
      if (removed) {
        toastContext.actions.showToast("Odoo connection removed", "success");
      }
    } catch (err) {
      toastContext.actions.showToast(
        `Failed to disconnect: ${errorText(err, "unknown error")}`,
        "danger"
      );
    }
  };

  const fields: Omit<FieldCardProps, "isCopied" | "onCopy">[] = [
    {
      label: "IP Address",
      value: appData.ipAddress,
      title: "Network IP Address",
      iconBg: "bg-blue-50 text-blue-600 border-blue-100/60",
      icon: <IpIcon className="w-3.5 h-3.5" />,
    },
    {
      label: "Obox Serial Number",
      value: appData.appId,
      title: "App ID (Hardware Serial)",
      iconBg: "bg-purple-50 text-odoo border-purple-100/60",
      icon: <AppIdIcon className="w-3.5 h-3.5" />,
    },
  ];

  const [closeSignal, setCloseSignal] = useState(0);

  useEffect(() => {
    if (isConnected) {
      setCloseSignal((c) => c + 1);
    }
  }, [isConnected]);

  const openButton = (
    <div
      role="button"
      tabIndex={0}
      title={"Click here to get more details"}
      className={`flex-1 min-w-0 h-full flex items-center justify-center gap-2 border-2 border-dashed rounded-lg px-2.5 py-3 cursor-pointer transition-colors ${isConnected
        ? "text-odoo border-odoo bg-gray-50 hover:border-odoo-dark hover:bg-gray-100"
        : "text-gray-600 border-gray-300 bg-gray-50 hover:border-gray-400 hover:bg-gray-100"
        }`}
    >
      {isConnected ? (
        <>
          <span className="truncate min-w-0 text-gray-600 text-sm">Connected with Odoo</span>
          <StatusPills lanStatus={lanStatus} wsStatus={wsStatus} />
          <DisconnectButton onClick={handleDisconnect} />
        </>
      ) : (
        <>
          <LinkIcon className="w-4 h-4 shrink-0" />
          <span className="truncate min-w-0 text-gray-600 text-sm">Connect with Odoo</span>
        </>
      )}
    </div>
  );

  return (
    <Dialog
      title="Your Obox App"
      openButton={openButton}
      closeSignal={closeSignal}
      maxWidth="max-w-md"
      showTitleDivider
    >
      <div className="space-y-4">
        {/* Connection Status Section */}
        {isConnected && odooData.status?.dbUrl ? (
          <div className="bg-purple-50/60 border border-purple-100 rounded-xl p-3 space-y-2.5">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-1.5 text-sm font-medium text-emerald-700">
                <span className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse" />
                Connected with Odoo
              </div>
              <div className="flex items-center gap-1.5">
                <StatusPills lanStatus={lanStatus} wsStatus={wsStatus} />
                <DisconnectButton
                  onClick={handleDisconnect}
                />
              </div>
            </div>

            <div className="flex items-center justify-between gap-2 bg-white/90 hover:bg-white border border-purple-100 hover:border-purple-200 rounded-lg px-2.5 py-1.5 min-w-0 transition-colors">
              <button
                type="button"
                onClick={() => {
                  if (odooData.status?.dbUrl) {
                    BrowserOpenURL(odooData.status.dbUrl);
                  }
                }}
                title="Open in browser"
                className="flex items-center gap-2 min-w-0 flex-1 text-left cursor-pointer group"
              >
                <LinkIcon className="w-3.5 h-3.5 text-odoo shrink-0 group-hover:scale-110 transition-transform" />
                <span className="font-mono text-xs text-gray-800 font-medium truncate group-hover:text-odoo group-hover:underline">
                  {odooData.status.dbUrl}
                </span>
              </button>
              <CopyButton
                label="Database URL"
                isCopied={isCopied("Database URL")}
                onCopy={() => copy(odooData.status?.dbUrl || "", "Database URL")}
              />
            </div>
          </div>
        ) : (
          <div className="flex items-center justify-between bg-amber-50/70 border border-amber-200/80 rounded-xl px-3 py-2 text-xs text-amber-800">
            <div className="flex items-center gap-2">
              <span className="w-2 h-2 rounded-full bg-amber-500" />
              <span className="font-medium">Not connected to Odoo</span>
            </div>
            <StatusPills lanStatus={lanStatus} wsStatus={wsStatus} />
          </div>
        )}

        {/* Credentials Cards (App ID & Local IP) */}
        <div>
          <div className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">
            Device Information
          </div>
          <div className="grid grid-cols-1 gap-2">
            {fields.map((field) => (
              <FieldCard
                key={field.label}
                {...field}
                isCopied={isCopied(field.label)}
                onCopy={() => copy(field.value, field.label)}
              />
            ))}
          </div>
        </div>

        {/* Instructions to Connect Odoo and Obox App */}
        <div className="border-t border-gray-100 pt-3">
          <div className="text-xs font-semibold text-gray-700 uppercase tracking-wider mb-2 flex items-center gap-1.5">
            <HelpCircleIcon className="w-4 h-4 text-odoo" />
            How to Connect with Odoo
          </div>

          <div className="space-y-2.5 text-xs text-gray-600">
            <div className="bg-gray-50 rounded-xl p-3 border border-gray-100 space-y-2">
              <div className="font-medium text-gray-800">
                Connect via Obox App
              </div>
              <ol className="list-decimal list-inside space-y-1.5 text-gray-600 pl-0.5">
                <li>
                  Open your Odoo instance and navigate to the{" "}
                  <strong className="text-gray-800">Obox</strong>
                </li>
                <li>
                  Click the <strong className="text-gray-800">Connect</strong> button
                </li>
                <li>
                  Paste your{" "}
                  <strong className="text-gray-800">Ip Address</strong> and{" "}
                  <strong className="text-gray-800">Obox Serial Number</strong>
                </li>
                <li>
                  Click <strong className="text-gray-800">Connect</strong>, then wait for the Obox App to link automatically
                </li>
              </ol>
            </div>
          </div>
        </div>
      </div>
    </Dialog>
  );
}
