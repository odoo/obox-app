import { useContext } from "react";
import BluetoothDialog from "./BluetoothDialog";
import NetworkDialog from "./NetworkDialog";
import PrinterListItem from "./PrinterListItem";
import { PrinterContext } from "../contexts/PrinterContext";

export default function PrinterList() {
  const { data } = useContext(PrinterContext);
  const { printers, unavailablePrinters, isLoading, errorMsg } = data;
  const hasPrinters = printers.length > 0 || unavailablePrinters.length > 0;

  return (
    <>
      <div className="w-full max-w-full sm:max-w-md md:max-w-lg lg:max-w-xl bg-white/85 rounded-2xl shadow-lg overflow-hidden px-4 sm:px-6 py-2 sm:py-4">
        {hasPrinters && (
          <div className="p-6">
            <ul className="divide-y divide-gray-300">
              {printers.map((printer) => (
                <PrinterListItem
                  key={printer.id}
                  printer={printer}
                  isOnline={true}
                />
              ))}
              {unavailablePrinters.map((printer) => (
                <PrinterListItem
                  key={printer.name}
                  printer={printer}
                  isOnline={false}
                />
              ))}
            </ul>
          </div>
        )}

        {isLoading ? (
          <div className="p-6">
            <div className="font-medium text-lg text-center">
              Searching for printers...
            </div>
          </div>
        ) : (
          !hasPrinters && (
            <div className="p-6">
              <div className="font-medium text-lg text-center">
                No printers found
              </div>
              <div className="mt-2 text-gray-600 text-center">
                Connect a powered-on printer via USB or add a network or Bluetooth printer below.
              </div>
            </div>
          )
        )}

        {errorMsg && (
          <div>
            <div className="text-red-700 mt-4 text-center">
              Error: {errorMsg}
            </div>
          </div>
        )}
      </div>

      <div className="mt-6 w-full max-w-full sm:max-w-md md:max-w-lg lg:max-w-xl flex flex-col sm:flex-row gap-3">
        <div className="flex-1">
          <NetworkDialog />
        </div>
        <div className="flex-1">
          <BluetoothDialog />
        </div>
      </div>
    </>
  );
}
