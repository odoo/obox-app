import { useContext, useEffect, useRef, useState } from "react";
import { GetTroubleshootInfo } from "../../wailsjs/go/main/App";
import { staticIpAdvice } from "../assets/data/troubleshootStep";
import { AppContext } from "../contexts/AppContext";
import { renderFormattedText } from "../functions/renderFormattedText";
import Dialog, { type ActionType } from "./Dialog";

export default function NetworkPrintingEnabledDialog() {
  const [openSignal, setOpenSignal] = useState(0);
  const [localIp, setLocalIp] = useState<string | null>(null);
  const isInitialMount = useRef(true);
  const appContext = useContext(AppContext);
  const { networkPrintingEnabled, isLoading } = appContext.data;
  const prevEnabled = useRef(networkPrintingEnabled);

  useEffect(() => {
    if (isLoading) {
      return;
    }

    if (isInitialMount.current) {
      isInitialMount.current = false;
      prevEnabled.current = networkPrintingEnabled;
      return;
    }

    if (!prevEnabled.current && networkPrintingEnabled) {
      setOpenSignal((count) => count + 1);
      GetTroubleshootInfo()
        .then((info) => setLocalIp(info?.localIp))
        .catch((err) => console.error("Failed to load troubleshoot info", err));
    }

    prevEnabled.current = networkPrintingEnabled;
  }, [networkPrintingEnabled, isLoading]);

  return (
    <Dialog
      title="Allow Other Devices to Print"
      openSignal={openSignal}
      actions={[{ name: "ok", label: "Got it", variant: "primary" as ActionType }]}
    >
      <p className="text-gray-600 text-sm leading-relaxed">
        Devices on this network will be able to send print jobs to your printers.
      </p>
      {localIp && (
        <p className="text-gray-600 text-sm whitespace-pre-line leading-relaxed mt-3">
          {renderFormattedText(staticIpAdvice(localIp))}
        </p>
      )}
    </Dialog>
  );
}
