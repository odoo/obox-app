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
    <>
      <div className="w-full max-w-full sm:max-w-md md:max-w-lg lg:max-w-xl bg-white/85 rounded-2xl shadow-lg overflow-hidden px-4 sm:px-6 py-2 sm:py-4">
        <OdooStatus />
        {printers && (printers.printers.length > 0 || printers.unavailablePrinters.length > 0) && (
          <div className="p-6">
            <ul className="divide-y divide-gray-300">
              {printers.printers.map((printer) => (
                <PrinterListItem
                  key={printer.identifier}
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
            <div className="p-6">
              <div className="font-medium text-lg text-center">
                Searching for printers...
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
          <div>
            <div className="text-red-700 mt-4 text-center">
              Error: {errorMessage}
            </div>
          </div>
        )}
      </div>

      <div className="mt-6 w-full max-w-full sm:max-w-md md:max-w-lg lg:max-w-xl">
        <NetworkIpDialog />
      </div>
    </>
  );
}
