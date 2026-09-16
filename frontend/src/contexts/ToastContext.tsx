import { createContext, useCallback, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useMountTransition } from "../hooks/useMountTransition";
import { BellIcon } from "../functions/icon";
import type { ToastType } from "../types";

type ToastContextType = {
  setters: {};
  data: {};
  actions: {
    showToast: (message: string, type?: ToastType) => void;
  };
};

export const ToastContext = createContext({} as ToastContextType);

interface ToastContextWrapper {
  children: React.ReactNode;
}

interface Toast {
  show: boolean;
  message: string;
  type: ToastType;
}

export const ToastContextWrapper = ({ children }: ToastContextWrapper) => {
  const toastTimeout = useRef<number | null>(null);
  const [toast, setToast] = useState<Toast>({
    show: false,
    message: "",
    type: "success",
  });

  const { mounted, visible } = useMountTransition(toast.show, 200);

  const showToast = useCallback(
    (message: string, type: ToastType = "success") => {
      if (toastTimeout.current) {
        clearTimeout(toastTimeout.current);
      }

      setToast({ show: true, message, type });
      toastTimeout.current = window.setTimeout(
        () => setToast((current) => ({ ...current, show: false })),
        type === "success" ? 2000 : 3000,
      );
    },
    [],
  );

  const data = {};
  const setters = {};
  const actions = {
    showToast,
  };

  return (
    <>
      <ToastContext.Provider value={{ data, setters, actions }}>
        {children}
      </ToastContext.Provider>

      {mounted &&
        createPortal(
          <div
            className={`fixed top-4 right-4 z-50 flex items-center gap-2.5 px-4 py-3 rounded-lg shadow-lg text-white text-sm max-w-xs transition-all duration-200 ${
              toast.type === "success" ? "bg-success" : "bg-danger"
            } ${
              visible
                ? "opacity-100 translate-x-0 ease-out"
                : "opacity-0 translate-x-4 ease-in pointer-events-none"
            }`}
          >
            <BellIcon className="w-4 h-4 shrink-0" />
            <span className="leading-snug">{toast.message}</span>
          </div>,
          document.body,
        )}
    </>
  );
};
