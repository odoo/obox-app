import { printer } from "../../wailsjs/go/models";
import { PrinterContext } from "../contexts/PrinterContext";
import { useContext } from "react";
import PrinterActions from "./PrinterActions";
import LibusbFixDialog from "./LibusbFixDialog";
import { PrinterTypeIcon, TrashIcon } from "../functions/icon";

type PrinterListItemProps =
  | {
      printer: printer.Device;
      isOnline: true;
    }
  | {
      printer: printer.UnavailableDevice;
      isOnline: false;
    };

export default function PrinterListItem({
  printer,
  isOnline,
}: PrinterListItemProps) {
  const printerContext = useContext(PrinterContext);

  const getPrinterStatusClass = (printer: printer.Device) => {
    if (!printer.isLAN) {
      return printer.online ? "text-success" : "text-danger";
    }
    const status = printer.lanIp
      ? printerContext.data.lanStatus[printer.lanIp]
      : undefined;

    if (status === "online") {
      return "text-success";
    }

    if (status === "offline") {
      return "text-danger";
    }

    return "text-info";
  };

  const hasLibUsbErrorFix = (error = "") => {
    return error.toLowerCase().includes("libusb");
  };

  return (
    <>
      {isOnline ? (
        <li
          key={printer.id}
          className="text-left first:pt-0 py-6 last:pb-0"
        >
          <div className="flex items-center justify-between gap-2">
            <span className={`shrink-0 ${getPrinterStatusClass(printer)}`}>
              <PrinterTypeIcon printer={printer} className="w-5 h-5" />
            </span>
            <span className="min-w-0 font-medium text-gray-900 break-all flex-1">
              {printer.name}
            </span>
            {printer.isLAN && (
              <button
                type="button"
                onClick={() => printerContext.actions.removeLanPrinter(printer)}
                title="Remove network printer"
                aria-label="Remove network printer"
                className="w-7 h-7 rounded-lg text-gray-400 hover:text-rose-600 hover:bg-rose-50 flex items-center justify-center transition-colors cursor-pointer"
              >
                <TrashIcon className="w-4 h-4" />
              </button>
            )}
          </div>
          <div className="text-gray-600 mt-2 text-sm break-all">
            {printer.ip}
          </div>
          <PrinterActions printer={printer} />
        </li>
      ) : (
        <li
          key={printer.name}
          className="text-left first:pt-0 py-6 last:pb-0"
        >
          <div className="flex items-center gap-2">
            <span className="w-3 h-3 rounded-full shrink-0 bg-danger" />
            <span className="min-w-0 font-medium text-gray-900">
              {printer.name}
            </span>
          </div>
          <div className="text-danger mt-1 text-wrap">
            Unable to communicate with this printer: {printer.errorMsg}
          </div>
          {hasLibUsbErrorFix(printer.errorMsg) && <LibusbFixDialog printerName={printer.name} />}
        </li>
      )}
    </>
  );
}
