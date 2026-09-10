import { printer } from "../../wailsjs/go/models";
import { PrinterContext } from "../contexts/PrinterContext";
import { useContext } from "react";
import PrinterActions from "./PrinterActions";
import LibusbFixDialog from "./LibusbFixDialog";
import CloseButton from "./CloseButton";

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
      return printer.online ? "bg-success" : "bg-danger";
    }
    const status = printer.lanIp
      ? printerContext.data.lanStatus[printer.lanIp]
      : undefined;

    if (status === "online") {
      return "bg-success";
    }

    if (status === "offline") {
      return "bg-danger";
    }

    return "bg-warning";
  };

  const hasLibUsbErrorFix = (error = "") => {
    return error.toLowerCase().includes("libusb");
  };

  return (
    <>
      {isOnline ? (
        <li
          key={printer.identifier}
          className="text-left first:pt-0 py-6 last:pb-0 relative"
        >
          <div className="flex items-center justify-between gap-2">
            <span
              className={`w-3 h-3 rounded-full shrink-0 ${getPrinterStatusClass(printer)}`}
            />
            <span className="min-w-0 font-medium text-gray-900 break-all flex-1">
              {printer.name}
            </span>
            {printer.isLAN && (
              <CloseButton
                onClick={() => printerContext.actions.removeLanPrinter(printer)}
              />
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
          className="text-left first:pt-0 py-6 last:pb-0 relative"
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
