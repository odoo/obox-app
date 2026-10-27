import { useContext } from "react";
import NetworkIpDialog from "./NetworkIpDialog";
import PrinterListItem from "./PrinterListItem";
import OdooStatus from "./OdooStatus";
import { PrinterContext } from "../contexts/PrinterContext";

export default function PrinterList() {
  const printerContext = useContext(PrinterContext);
  const { printers, fetchError } = printerContext.data;
  const errorMessage = fetchError ?? printers?.errorMsg;

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
            <div className="p-4 sm:p-6 divide-y divide-gray-200 animate-pulse">
              <div className="first:pt-0 pb-6 space-y-3">
                <div className="flex items-center gap-2.5">
                  <div className="w-5 h-5 rounded bg-gray-200 shrink-0" />
                  <div className="h-4 bg-gray-200 rounded w-44" />
                </div>
                <div className="h-3 bg-gray-100 rounded w-28 ml-7.5" />
                <div className="flex gap-2 pt-1">
                  <div className="h-9 bg-gray-200 rounded-lg flex-1" />
                  <div className="h-9 bg-gray-100 rounded-lg flex-1" />
                </div>
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
      </div>
    </div>
  );
}
