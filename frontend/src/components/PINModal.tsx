import { useContext, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { WebViewContext } from "../contexts/WebViewContext";
import { ToastContext } from "../contexts/ToastContext";
import { AppContext } from "../contexts/AppContext";
import { usePref, KEYS } from "../hooks/useLocalStorage";

const MAX_ATTEMPTS = 3;
const COOLDOWN_SECONDS = 60;

interface PINModalProps {
  onSuccess: (pin?: string) => void;
  onDismiss: () => void;
  mode?: "auth" | "set";
  title?: string;
  subtitle?: string;
}

export default function PINModal({
  onSuccess,
  onDismiss,
  mode = "auth",
  title,
  subtitle,
}: PINModalProps) {
  const { actions } = useContext(WebViewContext);
  const appContext = useContext(AppContext);
  const toastContext = useContext(ToastContext);

  const isLocalhost =
    typeof window !== "undefined" &&
    (window.location.hostname === "127.0.0.1" ||
      window.location.hostname === "localhost");

  const [digits, setDigits] = useState<string[]>([]);
  const [attempts, setAttempts] = usePref<number>(KEYS.PIN_ATTEMPTS, 0);
  const [cooldown, setCooldown] = usePref<number>(KEYS.PIN_COOLDOWN_UNTIL, 0);
  const [error, setError] = useState<string | null>(() => {
    if (mode !== "auth") return null;
    if (cooldown > 0) {
      return `Too many attempts. Wait ${cooldown}s.`;
    }
    if (attempts > 0 && attempts < MAX_ATTEMPTS) {
      return `Incorrect PIN (${MAX_ATTEMPTS - attempts} attempt${
        MAX_ATTEMPTS - attempts === 1 ? "" : "s"
      } left)`;
    }
    return null;
  });
  const [shaking, setShaking] = useState(false);
  const cooldownRef = useRef<number | null>(null);

  // For "set" mode
  const [step, setStep] = useState<"enter" | "confirm">("enter");
  const [firstPin, setFirstPin] = useState<string | null>(null);

  // Resume or start countdown timer if cooldown is active
  useEffect(() => {
    if (mode !== "auth" || cooldown <= 0) return;

    setError(`Too many attempts. Wait ${cooldown}s.`);
    cooldownRef.current = window.setInterval(() => {
      setCooldown((prev) => {
        const next = prev - 1;
        if (next <= 0) {
          if (cooldownRef.current !== null) {
            clearInterval(cooldownRef.current);
          }
          setAttempts(0);
          setError(null);
          return 0;
        }
        setError(`Too many attempts. Wait ${next}s.`);
        return next;
      });
    }, 1000);

    return () => {
      if (cooldownRef.current !== null) {
        clearInterval(cooldownRef.current);
      }
    };
  }, [mode, cooldown, setAttempts, setCooldown]);

  // keyboard listener
  useEffect(() => {
    const handleKey = (e: KeyboardEvent) => {
      if (cooldown > 0) return;
      if (e.key >= "0" && e.key <= "9") {
        addDigit(e.key);
      } else if (e.key === "Backspace") {
        removeDigit();
      } else if (e.key === "Escape") {
        onDismiss();
      }
    };
    window.addEventListener("keydown", handleKey);
    return () => window.removeEventListener("keydown", handleKey);
  }, [digits, cooldown, step, firstPin, onDismiss]);

  // auto-submit when 4 digits entered
  useEffect(() => {
    if (digits.length === 4 && cooldown === 0) {
      if (mode === "set") {
        handleSetSubmit(digits.join(""));
      } else {
        handleAuthSubmit(digits.join(""));
      }
    }
  }, [digits, mode, step, firstPin, cooldown]);

  const addDigit = (d: string) => {
    if (digits.length >= 4 || cooldown > 0) return;
    setDigits((prev) => [...prev, d]);
    setError(null);
  };

  const removeDigit = () => {
    if (digits.length > 0) {
      setDigits((prev) => prev.slice(0, -1));
      setError(null);
    } else if (mode === "set" && step === "confirm" && firstPin) {
      // Go back to the enter step
      setStep("enter");
      setDigits(firstPin.split(""));
      setFirstPin(null);
      setError(null);
    }
  };

  const shake = () => {
    setShaking(true);
    setTimeout(() => setShaking(false), 500);
  };

  const startCooldown = () => {
    setCooldown(COOLDOWN_SECONDS);
    setError(`Too many attempts. Wait ${COOLDOWN_SECONDS}s.`);
    if (cooldownRef.current !== null) {
      clearInterval(cooldownRef.current);
    }
    cooldownRef.current = window.setInterval(() => {
      setCooldown((prev) => {
        const next = prev - 1;
        if (next <= 0) {
          if (cooldownRef.current !== null) {
            clearInterval(cooldownRef.current);
          }
          setAttempts(0);
          setError(null);
          return 0;
        }
        setError(`Too many attempts. Wait ${next}s.`);
        return next;
      });
    }, 1000);
  };

  const handleAuthSubmit = async (pin: string) => {
    const ok = await actions.validatePIN(pin);
    if (ok) {
      setAttempts(0);
      setCooldown(0);
      onSuccess(pin);
      return;
    }

    shake();
    setDigits([]);
    const next = attempts + 1;
    setAttempts(next);
    if (next >= MAX_ATTEMPTS) {
      startCooldown();
    } else {
      setError(
        `Incorrect PIN (${MAX_ATTEMPTS - next} attempt${
          MAX_ATTEMPTS - next === 1 ? "" : "s"
        } left)`,
      );
    }
  };

  const handleSetSubmit = async (pin: string) => {
    if (step === "enter") {
      setFirstPin(pin);
      setStep("confirm");
      setDigits([]);
      setError(null);
    } else {
      if (pin === firstPin) {
        try {
          await actions.savePIN(pin);
          toastContext.actions.showToast(
            "Security PIN updated successfully",
            "success",
          );
          onSuccess(pin);
        } catch (err: unknown) {
          shake();
          setError(String(err) || "Failed to save PIN.");
          setDigits([]);
          setStep("enter");
          setFirstPin(null);
        }
      } else {
        shake();
        setError("PINs do not match. Try again.");
        setDigits([]);
        setStep("enter");
        setFirstPin(null);
      }
    }
  };

  const keys = ["1", "2", "3", "4", "5", "6", "7", "8", "9", "", "0", "⌫"];

  const modalTitle =
    mode === "set"
      ? step === "enter"
        ? "Set New PIN"
        : "Confirm PIN"
      : title || "Enter PIN";

  const modalSubtitle =
    mode === "set"
      ? step === "enter"
        ? "Enter 4-digit code"
        : "Re-enter to confirm"
      : subtitle || "4-digit unlock code";

  return createPortal(
    <div className="fixed inset-0 z-[9999] flex items-center justify-center bg-black/80 backdrop-blur-sm">
      <div
        className={`bg-white rounded-3xl shadow-2xl p-8 w-72 flex flex-col items-center gap-6 select-none ${
          shaking ? "animate-[shake_0.5s_ease-in-out]" : ""
        }`}
        style={shaking ? { animation: "shake 0.5s ease-in-out" } : {}}
      >
        {/* Title */}
        <div className="text-center">
          <div className="text-2xl font-semibold text-gray-800 mb-1">
            {modalTitle}
          </div>
          <div className="text-sm text-gray-400">{modalSubtitle}</div>
        </div>

        {/* Dot display */}
        <div className="flex gap-3">
          {Array.from({ length: 4 }).map((_, i) => (
            <div
              key={i}
              className={`w-4 h-4 rounded-full border-2 transition-all duration-150 ${
                i < digits.length
                  ? "bg-odoo border-odoo scale-110"
                  : "border-gray-300 bg-transparent"
              }`}
            />
          ))}
        </div>

        {/* Error / cooldown */}
        {error && (
          <div className="text-sm text-red-500 text-center -mt-2">
            {cooldown > 0 ? `Too many attempts. Wait ${cooldown}s.` : error}
          </div>
        )}

        {/* Numpad */}
        <div className="grid grid-cols-3 gap-3 w-full">
          {keys.map((k, i) => {
            if (k === "") return <div key={i} />;

            const isDelete = k === "⌫";
            const disabled = cooldown > 0 || (digits.length >= 4 && !isDelete);

            return (
              <button
                key={i}
                disabled={disabled}
                onClick={() => (isDelete ? removeDigit() : addDigit(k))}
                className={`h-14 rounded-2xl text-lg font-medium transition-all duration-100 cursor-pointer
                  ${
                    isDelete
                      ? "bg-gray-100 text-gray-500 hover:bg-gray-200 active:scale-95"
                      : "bg-gray-50 text-gray-800 hover:bg-odoo/10 hover:text-odoo active:scale-95 active:bg-odoo/20"
                  }
                  disabled:opacity-40 disabled:cursor-not-allowed`}
              >
                {k}
              </button>
            );
          })}
        </div>

        {/* Cancel */}
        {(appContext.data.isWails || isLocalhost) && (
          <button
            onClick={onDismiss}
            className="text-sm text-gray-400 hover:text-gray-600 cursor-pointer transition-colors"
          >
            Cancel
          </button>
        )}
      </div>

      <style>{`
        @keyframes shake {
          0%,100%{transform:translateX(0)}
          15%{transform:translateX(-8px)}
          30%{transform:translateX(8px)}
          45%{transform:translateX(-6px)}
          60%{transform:translateX(6px)}
          75%{transform:translateX(-3px)}
          90%{transform:translateX(3px)}
        }
      `}</style>
    </div>,
    document.body,
  );
}
