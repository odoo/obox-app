import { useContext, useEffect, useRef, useState } from "react";
import { PrinterContext } from "../contexts/PrinterContext";
import { ToastContext } from "../contexts/ToastContext";
import Dialog from "./Dialog";
import {
  BluetoothIcon,
  CheckIcon,
  PrinterIcon,
  SearchIcon,
  WarningIcon,
} from "../functions/printerIcons";
import {
  CheckBluetoothDependencies,
  ScanBluetoothPrinters,
} from "../../wailsjs/go/main/App";
import { bluetooth } from "../../wailsjs/go/models";

export default function BluetoothDialog() {
  const printerContext = useContext(PrinterContext);
  const toastContext = useContext(ToastContext);

  const [macInput, setMacInput] = useState("");
  const [nameInput, setNameInput] = useState("");
  const [isEscpos, setIsEscpos] = useState(true);
  const [bottomPadding, setBottomPadding] = useState(0);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [scanError, setScanError] = useState<string | null>(null);
  const [isScanning, setIsScanning] = useState(false);
  const [devices, setDevices] = useState<bluetooth.BluetoothPrinterInfo[]>([]);
  const [selectedMac, setSelectedMac] = useState("");
  const [dependencies, setDependencies] = useState<bluetooth.DependencyStatus[]>([]);
  const [copiedCmd, setCopiedCmd] = useState<string | null>(null);

  const checkDependenciesAndScan = async () => {
    try {
      const missing = await CheckBluetoothDependencies();
      if (missing && missing.length > 0) {
        setDependencies(missing);
        return;
      }
      setDependencies([]);
      await handleScan();
    } catch (err) {
      console.error("Failed to check Bluetooth dependencies:", err);
    }
  };

  const handleScan = async () => {
    setIsScanning(true);
    setScanError(null);
    setDevices([]);
    try {
      const found = await ScanBluetoothPrinters();
      setDevices(found || []);
      if (!found || found.length === 0) {
        setScanError(
          "No paired Bluetooth devices found. Pair your printer first in system Bluetooth settings.",
        );
      }
    } catch (err) {
      setScanError(
        typeof err === "string" ? err : (err as Error)?.message || "Scan failed",
      );
    } finally {
      setIsScanning(false);
    }
  };

  const handleSelectDevice = (device: bluetooth.BluetoothPrinterInfo) => {
    setSelectedMac(device.address);
    setMacInput(device.address);
    if (!nameInput) {
      setNameInput(device.name);
    }
    setErrorMessage(null);
  };

  const copyCommand = (cmd: string) => {
    navigator.clipboard.writeText(cmd);
    setCopiedCmd(cmd);
    setTimeout(() => setCopiedCmd(null), 2000);
  };

  const cleanup = () => {
    setMacInput("");
    setNameInput("");
    setIsEscpos(true);
    setBottomPadding(0);
    setErrorMessage(null);
    setScanError(null);
    setDevices([]);
    setSelectedMac("");
    setDependencies([]);
  };

  const submit = async () => {
    const mac = (selectedMac || macInput).trim();
    if (!mac) {
      setErrorMessage("Please select a device or enter a Bluetooth MAC / address");
      return false;
    }

    if (!isEscpos && (isNaN(bottomPadding) || bottomPadding < 0 || bottomPadding > 500)) {
      setErrorMessage("Bottom padding must be between 0 and 500");
      return false;
    }

    const name = nameInput.trim() || mac;
    const result = await printerContext.actions.addPrinter({
      connectionType: "bluetooth",
      address: mac,
      name,
      protocol: isEscpos ? "ESCPOS" : "ESCPOS_PARTIAL",
      bottomPadding: !isEscpos ? bottomPadding : 0,
    });

    if (!result.status) {
      setErrorMessage(result.message);
      return false;
    }

    toastContext.actions.showToast(result.message, "success");
    return true;
  };

  return (
    <Dialog
      title="Add Bluetooth Printer"
      onOpen={checkDependenciesAndScan}
      onClose={cleanup}
      actions={[
        {
          name: "submit",
          label: "Add Printer",
          disabled: !macInput.trim() && !selectedMac,
          onClick: submit,
          variant: "primary",
        },
      ]}
      openButton={
        <div className="w-full border-2 border-dashed border-gray-300 bg-gray-50 rounded-lg px-4 py-3 text-center text-gray-600 hover:border-gray-400 hover:bg-gray-100 cursor-pointer flex items-center justify-center gap-2 transition-colors">
          <BluetoothIcon className="w-4 h-4 shrink-0 text-gray-500" />
          <span>Add Bluetooth Printer</span>
        </div>
      }
    >
      {/* Dependencies Warning */}
      {dependencies.length > 0 && (
        <div className="mb-4 bg-amber-50 border border-amber-200 rounded-lg p-3 text-xs text-amber-800">
          <div className="font-semibold flex items-center gap-1.5 mb-1.5 text-amber-950">
            <WarningIcon className="w-4 h-4 shrink-0 inline-block text-amber-600" />
            <span>Missing Bluetooth Dependencies</span>
          </div>
          <div className="space-y-2">
            {dependencies.map((dep) => (
              <div
                key={dep.name}
                className="border-t border-amber-200/50 pt-1.5 first:border-0 first:pt-0"
              >
                <div className="font-semibold text-amber-950">{dep.name}</div>
                <div className="text-amber-700 mb-1.5">{dep.description}</div>
                <div className="bg-amber-950/5 text-amber-900 px-2 py-1 rounded font-mono select-all flex justify-between items-center gap-1">
                  <span className="truncate">{dep.installCmd}</span>
                  <button
                    type="button"
                    onClick={() => copyCommand(dep.installCmd)}
                    className="text-[10px] uppercase text-amber-600 font-sans tracking-wider font-semibold cursor-pointer hover:text-amber-800 shrink-0"
                  >
                    {copiedCmd === dep.installCmd ? "Copied" : "Copy"}
                  </button>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Scan Button */}
      <button
        type="button"
        onClick={handleScan}
        disabled={isScanning}
        className="w-full border rounded-lg px-4 py-2 mb-4 text-sm cursor-pointer flex items-center justify-center gap-2 border-stone-300 text-stone-700 hover:bg-stone-50 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
      >
        {isScanning ? (
          <span className="animate-spin inline-block w-4 h-4 border-2 border-current border-t-transparent rounded-full" />
        ) : (
          <SearchIcon className="w-4 h-4 shrink-0" />
        )}
        <span>{isScanning ? "Scanning..." : "Scan for Devices"}</span>
      </button>

      {/* Scan error */}
      {scanError && (
        <div className="text-danger text-xs mb-3 text-center">{scanError}</div>
      )}

      {/* Device list from scan */}
      {devices.length > 0 && (
        <div className="mb-4 border border-gray-200 rounded-lg overflow-hidden">
          <div className="text-xs text-gray-500 px-3 pt-2 pb-1 font-medium uppercase tracking-wide bg-gray-50 flex justify-between items-center">
            <span>Paired Devices</span>
            <span className="text-[11px] normal-case text-gray-400">
              {devices.filter((d) => d.device === "printer").length} printer(s) detected
            </span>
          </div>
          <ul className="divide-y divide-gray-100 max-h-48 overflow-y-auto">
            {devices.map((device) => {
              const isSelected = selectedMac === device.address;
              const isPrinter = device.device === "printer";
              return (
                <li
                  key={device.address}
                  onClick={() => handleSelectDevice(device)}
                  className={`flex items-center gap-3 px-3 py-2.5 cursor-pointer hover:bg-blue-50 transition-colors ${isSelected ? "bg-blue-50 ring-1 ring-inset ring-blue-400" : ""
                    }`}
                >
                  {isPrinter ? (
                    <PrinterIcon className="w-4 h-4 shrink-0 text-emerald-600" />
                  ) : (
                    <BluetoothIcon className="w-4 h-4 shrink-0 text-gray-400" />
                  )}
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-1.5">
                      <span className="text-sm font-medium text-gray-800 truncate">
                        {device.name}
                      </span>
                      {isPrinter ? (
                        <span className="text-[10px] bg-emerald-50 text-emerald-700 border border-emerald-200 px-1.5 py-0.5 rounded font-medium shrink-0">
                          Printer
                        </span>
                      ) : (
                        <span
                          title="Warning: This device might not be a printer. Please verify before adding."
                          className="inline-flex items-center text-amber-500 hover:text-amber-600 cursor-help shrink-0"
                          onClick={(e) => e.stopPropagation()}
                        >
                          <WarningIcon className="w-3.5 h-3.5" />
                        </span>
                      )}
                    </div>
                    <div className="text-xs text-gray-400 font-mono truncate">
                      {device.address}
                    </div>
                  </div>
                  {isSelected && (
                    <CheckIcon className="w-4 h-4 shrink-0 text-blue-500" />
                  )}
                </li>
              );
            })}
          </ul>
        </div>
      )}

      {/* Manual Input Fields */}
      <div className="space-y-3 mb-2">
        <div>
          <label className="block text-xs font-medium text-gray-700 mb-1">
            Device Address (MAC or UUID)
          </label>
          <input
            type="text"
            value={macInput}
            onChange={(e) => {
              setMacInput(e.target.value);
              setSelectedMac("");
              setErrorMessage(null);
            }}
            placeholder="00:11:22:33:44:55 or BLE UUID"
            className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-odoo focus:border-transparent font-mono"
          />
        </div>

        <div>
          <label className="block text-xs font-medium text-gray-700 mb-1">
            Printer Name (Optional)
          </label>
          <input
            type="text"
            value={nameInput}
            onChange={(e) => setNameInput(e.target.value)}
            placeholder="e.g. Bluetooth Receipt Printer"
            className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-odoo focus:border-transparent"
          />
        </div>

        <div className="pt-1">
          <label className="flex items-center justify-between cursor-pointer">
            <span className="text-xs font-medium text-gray-700">ESCPOS</span>
            <div className="relative inline-flex items-center">
              <input
                type="checkbox"
                checked={isEscpos}
                onChange={(e) => setIsEscpos(e.target.checked)}
                className="sr-only peer"
              />
              <div className="w-9 h-5 bg-gray-200 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-odoo"></div>
            </div>
          </label>

          {!isEscpos && (
            <div className="mt-3">
              <label className="block text-xs font-medium text-gray-700 mb-1">
                Bottom Padding (px)
              </label>
              <input
                type="number"
                min={0}
                max={500}
                value={bottomPadding}
                onChange={(e) => {
                  const val = e.target.value === "" ? 0 : Number(e.target.value);
                  setBottomPadding(val);
                  setErrorMessage(null);
                }}
                placeholder="0"
                className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-odoo focus:border-transparent"
              />
              <span className="text-[11px] text-gray-500 mt-1 block">
                Extra space added at the end of partial receipt (0 - 500 px)
              </span>
            </div>
          )}
        </div>
      </div>

      {errorMessage && (
        <div
          className="bg-red-100 border border-red-400 text-red-700 rounded-md text-sm px-4 mb-3 py-2 relative mt-3"
          role="alert"
        >
          {errorMessage}
        </div>
      )}
    </Dialog>
  );
}
