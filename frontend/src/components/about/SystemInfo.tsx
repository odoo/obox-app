import { useContext } from "react";
import { AppContext } from "../../contexts/AppContext";
import { useClipboard } from "../../hooks/useClipboard";
import {
  CheckIcon,
  CopyIcon,
} from "../../functions/icon";
import appIcon from "@build/appicon.png";

interface InfoRowProps {
  label: string;
  isLoading: boolean;
  skeletonWidth?: string;
  value?: React.ReactNode;
  copyValue?: string;
  copyKey?: string;
  tooltip?: string;
}

function InfoRow({
  label,
  isLoading,
  skeletonWidth = "w-24",
  value,
  copyValue,
  copyKey,
  tooltip,
}: InfoRowProps) {
  const { copy, isCopied } = useClipboard();

  if (!isLoading && !value && !copyValue) {
    return null;
  }

  return (
    <div className="flex items-center justify-between px-3 py-2 gap-2" title={tooltip}>
      <span className="text-gray-500 shrink-0">{label}</span>
      {isLoading ? (
        <span className={`inline-block ${skeletonWidth} h-3.5 bg-gray-200 rounded animate-pulse`} />
      ) : copyValue ? (
        <div className="flex items-center gap-1.5 min-w-0">
          <span className="font-mono text-gray-700 text-[11px] truncate max-w-[170px]" title={copyValue}>
            {copyValue}
          </span>
          <button
            type="button"
            onClick={() => copy(copyValue, copyKey || label, label)}
            className="p-0.5 text-gray-400 hover:text-odoo rounded cursor-pointer shrink-0 transition-colors"
            title={`Copy ${label}`}
          >
            {isCopied(copyKey || label) ? (
              <CheckIcon className="w-3.5 h-3.5 text-emerald-600" />
            ) : (
              <CopyIcon className="w-3.5 h-3.5" />
            )}
          </button>
        </div>
      ) : (
        value
      )}
    </div>
  );
}

export default function SystemInfo() {
  const appContext = useContext(AppContext);
  const isLoading = appContext.data.isLoading;

  const {
    appId,
    server: { ipAddress, port },
    system: { os, arch, goVersion, hostname },
    build: { version, buildTime, commit, dev },
    settings: { debugMode, networkPrintingEnabled },
  } = appContext.data;

  const osName = os === "windows" ? "Windows" : os === "darwin" ? "macOS" : os === "linux" ? "Linux" : os || "Unknown";
  const platformDisplay = arch ? `${osName} (${arch})` : osName;
  const serverAddress = ipAddress
    ? (ipAddress.startsWith("http") ? ipAddress : `http://${ipAddress}`)
    : (port ? `http://127.0.0.1:${port}` : "");

  const formattedBuildTime = (() => {
    if (!buildTime || buildTime === "unknown") return null;
    try {
      const d = new Date(buildTime);
      return isNaN(d.getTime()) ? buildTime : d.toLocaleDateString(undefined, {
        year: "numeric",
        month: "short",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      });
    } catch {
      return buildTime;
    }
  })();

  return (
    <>
      {/* App Branding Header */}
      <div className="flex items-center gap-3.5 pb-0.5">
        <div className="w-12 h-12 rounded-xl bg-odoo/10 flex items-center justify-center shrink-0 border border-odoo/20 shadow-2xs">
          <img src={appIcon} alt="Obox App" className="w-8 h-8 object-contain" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="text-base font-semibold text-gray-900 leading-tight">Obox App</span>
            {isLoading ? (
              <span className="inline-block w-12 h-4 bg-gray-200 rounded animate-pulse" />
            ) : (
              <span className="font-mono text-[10px] font-semibold text-odoo bg-odoo/10 px-1.5 py-0.5 rounded border border-odoo/20">
                {version}
              </span>
            )}
            {!isLoading && debugMode && (
              <span className="text-[10px] font-semibold text-purple-700 bg-purple-100 px-1.5 py-0.5 rounded border border-purple-200">
                DEBUG
              </span>
            )}
          </div>
          <div className="text-xs text-gray-500 mt-0.5">Odoo POS Hardware & Printing Service</div>
        </div>
      </div>

      {/* System Details */}
      <div className="rounded-xl border border-gray-200/80 bg-gray-50/50 p-1 text-xs">
        <div className="divide-y divide-gray-200/60">
          <InfoRow
            label="App ID"
            isLoading={isLoading}
            skeletonWidth="w-32"
            copyValue={appId}
            copyKey="appId"
          />

          <InfoRow
            label="Server Address"
            isLoading={isLoading}
            skeletonWidth="w-32"
            copyValue={serverAddress}
            copyKey="serverAddress"
          />

          <InfoRow
            label="Platform"
            isLoading={isLoading}
            skeletonWidth="w-24"
            value={<span className="font-medium text-gray-800">{platformDisplay}</span>}
          />

          <InfoRow
            label="Device"
            isLoading={isLoading}
            skeletonWidth="w-24"
            value={
              hostname ? (
                <span className="font-mono text-gray-700 text-[11px] truncate max-w-[170px]" title={hostname}>
                  {hostname}
                </span>
              ) : null
            }
          />

          <InfoRow
            label="Runtime"
            isLoading={isLoading}
            skeletonWidth="w-20"
            value={goVersion ? <span className="font-mono text-gray-700 text-[11px]">{goVersion}</span> : null}
          />

          <InfoRow
            label="Network Mode"
            isLoading={isLoading}
            skeletonWidth="w-20"
            tooltip={
              networkPrintingEnabled
                ? "LAN Mode: Devices on your local network can discover and send print jobs to your printers."
                : "Local Only Mode: Only this computer can send print jobs. Print requests from other network devices are blocked."
            }
            value={
              <span
                className="font-medium text-gray-800 cursor-help border-b border-dotted border-gray-400/80"
                title={
                  networkPrintingEnabled
                    ? "LAN Mode: Devices on your local network can discover and send print jobs to your printers."
                    : "Local Only Mode: Only this computer can send print jobs. Print requests from other network devices are blocked."
                }
              >
                {networkPrintingEnabled ? "LAN (All Devices)" : "Local Only"}
              </span>
            }
          />

          <InfoRow
            label="Build Date"
            isLoading={isLoading}
            skeletonWidth="w-28"
            value={
              formattedBuildTime ? (
                <span className="text-gray-600 font-mono text-[11px]">{formattedBuildTime}</span>
              ) : null
            }
          />
        </div>
      </div>
    </>
  );
}
