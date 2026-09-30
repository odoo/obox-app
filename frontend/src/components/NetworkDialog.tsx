import { useContext, useRef, useState } from "react";
import { PrinterContext } from "../contexts/PrinterContext";
import Dialog, { ActionType } from "./Dialog";
import { WifiIcon } from "../functions/printerIcons";

const isValidOctet = (value: string) => {
  const number = Number(value);
  return Number.isInteger(number) && number >= 0 && number <= 255;
};

const extractIP = (text: string) => {
  const trimmed = text.trim();
  const match = trimmed.match(/^(?:https?:\/\/)?((?:\d{1,3}\.){3}\d{1,3})(?::\d+)?(?:\/.*)?$/);
  return match?.[1] ?? null;
};

export default function NetworkDialog() {
  const printerContext = useContext(PrinterContext);
  const [ipParts, setIpParts] = useState(["", "", "", ""]);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const inputRefs = useRef<(HTMLInputElement | null)[]>([]);

  const updatePart = (index: number, value: string) => {
    const sanitized = value.replace(/\D/g, "").slice(0, 3);

    setIpParts((parts) => {
      const next = [...parts];
      next[index] = sanitized;

      if (next.every((part) => Boolean(part))) {
        setErrorMessage(null);
      }

      return next;
    });

    if ((sanitized.length === 3 || (sanitized.length > 1 && Number(sanitized) > 25)) && index < 3) {
      inputRefs.current[index + 1]?.focus();
      inputRefs.current[index + 1]?.select();
    }
  };

  const handleKeyDown = (event: React.KeyboardEvent<HTMLInputElement>, index: number,) => {
    if (event.ctrlKey || event.metaKey) {
      return;
    }

    const allowedKeys = ["Backspace", "Delete", "ArrowLeft", "ArrowRight", "Tab",];

    if (!/[0-9]/.test(event.key) && !allowedKeys.includes(event.key)) {
      if (event.key === ".") {
        event.preventDefault();

        if (index < 3) {
          inputRefs.current[index + 1]?.focus();
        }
        return;
      }

      event.preventDefault();
    }

    if (event.key === "Backspace" && !ipParts[index] && index > 0) {
      inputRefs.current[index - 1]?.focus();
    }
  };

  const handlePaste = (event: React.ClipboardEvent) => {
    event.preventDefault();

    const pasted = event.clipboardData.getData("text").trim();
    const ip = extractIP(pasted);
    const parts = ip?.split(".");
    if (!parts || !parts.every(isValidOctet)) {
      setErrorMessage("The pasted address is not a valid IP address");
      return;
    }
    setIpParts(parts);
    setErrorMessage(null);
    inputRefs.current[3]?.focus();
  };

  const submit = async () => {
    if (!ipParts.every(isValidOctet)) {
      setErrorMessage("Please enter a valid IP address");
      return false;
    }

    const ip = ipParts.join(".");
    const result = await printerContext.actions.addPrinter({ connectionType: "lan", address: ip });
    if (!result.status) {
      setErrorMessage(result.message);
      return false;
    }

    return true;
  };

  const cleanup = () => {
    setIpParts(["", "", "", ""]);
    setErrorMessage(null);
  };

  return (
    <Dialog
      title="Add Network Printer"
      actions={[{ name: "submit", label: "Submit", disabled: ipParts.some((part) => !part), onClick: submit, variant: "primary" as ActionType }]}
      onClose={cleanup}
      openButton={
        <div className="w-full border-2 border-dashed border-gray-300 bg-gray-50 rounded-lg px-4 py-3 text-center text-gray-600 hover:border-gray-400 hover:bg-gray-100 cursor-pointer flex items-center justify-center gap-2 transition-colors">
          <WifiIcon className="w-4 h-4 shrink-0 text-gray-500" />
          <span>Add Network Printer</span>
        </div>
      }
    >
      <div
        className="flex items-center justify-center gap-2 mb-3"
        onPasteCapture={handlePaste}
      >
        {ipParts.map((part, index) => (
          <div key={index} className="flex items-center gap-2">
            <input
              ref={(element) => {
                inputRefs.current[index] = element;
                if (index === 0 && !ipParts.some((part) => part)) {
                  element?.focus();
                }
              }}
              value={part}
              type="text"
              inputMode="numeric"
              maxLength={3}
              className="w-16 border border-gray-300 rounded-lg px-2 py-2 text-center text-sm focus:outline-none focus:ring-1 focus:ring-odoo-light focus:border-transparent"
              onChange={(event) => updatePart(index, event.target.value)}
              onKeyDown={(event) => handleKeyDown(event, index)}
            />

            {index < 3 && (<span className="text-gray-400 font-medium">.</span>)}
          </div>
        ))}
      </div>

      {errorMessage && (
        <div
          className="bg-red-100 border border-red-400 text-red-700 rounded-md text-sm px-4 mb-3 py-3 relative"
          role="alert"
        >
          {errorMessage}
        </div>
      )}
    </Dialog>
  );
}
